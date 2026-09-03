package service

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/urso/nota/pkg/thread"
)

// fileSnapshot holds identity and modification state for a file.
type fileSnapshot struct {
	mtime time.Time
	size  int64
	inode uint64
}

// Poller watches .nota/ for changes and notifies subscribers.
type Poller struct {
	service  *Service
	interval time.Duration

	mu       sync.Mutex
	snapshot map[string]fileSnapshot
	running  bool
	paused   bool
	cancel   context.CancelFunc
}

// NewPoller creates a poller for the service with the given interval.
func NewPoller(svc *Service, interval time.Duration) *Poller {
	return &Poller{
		service:  svc,
		interval: interval,
		snapshot: make(map[string]fileSnapshot),
	}
}

// Start begins polling. Safe to call multiple times.
func (p *Poller) Start(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return
	}

	ctx, p.cancel = context.WithCancel(ctx)
	p.running = true
	p.paused = false

	go p.run(ctx)
}

// Stop halts the poller.
func (p *Poller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.running = false
}

// Pause stops polling but retains the snapshot for resume.
func (p *Poller) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = true
}

// Resume restarts polling after pause, emitting any accumulated changes.
func (p *Poller) Resume() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = false
}

// IsPaused returns whether the poller is paused.
func (p *Poller) IsPaused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

func (p *Poller) run(ctx context.Context) {
	// Initial scan to populate snapshot
	p.scan()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.mu.Lock()
			paused := p.paused
			p.mu.Unlock()

			if paused {
				continue
			}

			if !p.checkDirExists() {
				p.service.signalShutdown()
				return
			}

			p.scan()
		}
	}
}

func (p *Poller) checkDirExists() bool {
	_, err := os.Stat(p.service.notaDir)
	if err != nil {
		return false
	}
	_, err = os.Stat(p.service.repoRoot)
	return err == nil
}

func (p *Poller) scan() {
	current, err := p.readDir()
	if err != nil {
		return
	}

	p.mu.Lock()
	old := p.snapshot
	p.mu.Unlock()

	added, removed, modified := p.diff(old, current)

	if len(added) == 0 && len(removed) == 0 && len(modified) == 0 {
		p.mu.Lock()
		p.snapshot = current
		p.mu.Unlock()
		return
	}

	event := p.buildEvent(added, removed, modified)

	p.mu.Lock()
	p.snapshot = current
	p.mu.Unlock()

	if len(event.ThreadIDs) > 0 || len(event.Files) > 0 {
		p.service.notifyFromPoller(event)
	}
}

func (p *Poller) readDir() (map[string]fileSnapshot, error) {
	entries, err := os.ReadDir(p.service.notaDir)
	if err != nil {
		return nil, err
	}

	result := make(map[string]fileSnapshot)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".xml" {
			continue
		}

		path := filepath.Join(p.service.notaDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		snap := fileSnapshot{
			mtime: info.ModTime(),
			size:  info.Size(),
		}

		// Get inode on Unix systems
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			snap.inode = stat.Ino
		}

		result[path] = snap
	}

	return result, nil
}

func (p *Poller) diff(old, current map[string]fileSnapshot) (added, removed, modified []string) {
	for path, snap := range current {
		oldSnap, exists := old[path]
		if !exists {
			added = append(added, path)
		} else if snap.mtime != oldSnap.mtime || snap.size != oldSnap.size || snap.inode != oldSnap.inode {
			modified = append(modified, path)
		}
	}

	for path := range old {
		if _, exists := current[path]; !exists {
			removed = append(removed, path)
		}
	}

	return
}

func (p *Poller) buildEvent(added, removed, modified []string) ChangeEvent {
	var event ChangeEvent
	seen := make(map[string]bool)
	seenFiles := make(map[string]bool)

	addThread := func(path string) {
		t, err := thread.ReadThread(path)
		if err != nil {
			return
		}

		if !seen[t.ID] {
			seen[t.ID] = true
			event.ThreadIDs = append(event.ThreadIDs, t.ID)
		}

		for _, f := range filesFromThread(t) {
			if !seenFiles[f] {
				seenFiles[f] = true
				event.Files = append(event.Files, f)
			}
		}
	}

	for _, path := range added {
		addThread(path)
	}
	for _, path := range modified {
		addThread(path)
	}

	// For removed files, we can't read the thread anymore.
	// The old snapshot doesn't store thread ID, so we emit a generic event.
	// Clients should refresh their view when they receive any change event.
	if len(removed) > 0 && len(event.ThreadIDs) == 0 {
		event.ThreadIDs = []string{}
	}

	return event
}
