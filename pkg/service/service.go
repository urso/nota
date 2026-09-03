package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/urso/nota/pkg/git"
)

// DefaultPollInterval is the default polling interval.
const DefaultPollInterval = 500 * time.Millisecond

// Service provides thread operations for a repository.
type Service struct {
	repoRoot string
	notaDir  string

	mu            sync.Mutex
	subscribers   []chan ChangeEvent
	pendingWrites map[string]bool
	poller        *Poller
	shutdownCh    chan struct{}
}

// New creates a Service for the repository at the given path.
// If path is empty, uses the current directory.
func New(path string) (*Service, error) {
	if path == "" {
		path = "."
	}

	root, err := git.RepoRoot(path)
	if err != nil {
		return nil, fmt.Errorf("finding repository root: %w", err)
	}

	notaDir := filepath.Join(root, ".nota")
	if _, err := os.Stat(notaDir); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf(".nota directory not found in %s", root)
		}
		return nil, fmt.Errorf("checking .nota directory: %w", err)
	}

	return &Service{
		repoRoot:      root,
		notaDir:       notaDir,
		pendingWrites: make(map[string]bool),
		shutdownCh:    make(chan struct{}),
	}, nil
}

// RepoRoot returns the repository root path.
func (s *Service) RepoRoot() string {
	return s.repoRoot
}

// NotaDir returns the .nota directory path.
func (s *Service) NotaDir() string {
	return s.notaDir
}

// ShutdownCh returns a channel that is closed when the service should shut down.
func (s *Service) ShutdownCh() <-chan struct{} {
	return s.shutdownCh
}

// StartPoller starts the change poller with the given context and interval.
func (s *Service) StartPoller(ctx context.Context, interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.poller != nil {
		return
	}

	s.poller = NewPoller(s, interval)
	s.poller.Start(ctx)
}

// StopPoller stops the change poller.
func (s *Service) StopPoller() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.poller != nil {
		s.poller.Stop()
		s.poller = nil
	}
}

// Subscribe returns a channel that receives change events.
// The channel is buffered to prevent blocking the poller.
// Call Unsubscribe to stop receiving events and close the channel.
func (s *Service) Subscribe() chan ChangeEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan ChangeEvent, 16)
	s.subscribers = append(s.subscribers, ch)

	// Resume poller if this is the first subscriber
	if len(s.subscribers) == 1 && s.poller != nil {
		s.poller.Resume()
	}

	return ch
}

// Unsubscribe removes a subscriber channel and closes it.
func (s *Service) Unsubscribe(ch chan ChangeEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, sub := range s.subscribers {
		if sub == ch {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			close(ch)
			break
		}
	}

	// Pause poller if no subscribers remain
	if len(s.subscribers) == 0 && s.poller != nil {
		s.poller.Pause()
	}
}

// SubscriberCount returns the current number of subscribers.
func (s *Service) SubscriberCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subscribers)
}

// notify sends a change event to all subscribers (non-blocking).
func (s *Service) notify(e ChangeEvent) {
	s.mu.Lock()
	subs := make([]chan ChangeEvent, len(s.subscribers))
	copy(subs, s.subscribers)
	s.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- e:
		default:
			// Channel full, skip to avoid blocking
		}
	}
}

// notifyFromPoller is called by the poller to deliver change events.
// It filters out pending writes to avoid duplicate notifications.
func (s *Service) notifyFromPoller(e ChangeEvent) {
	s.mu.Lock()

	// Filter out thread IDs that were written by the service
	var filteredIDs []string
	for _, id := range e.ThreadIDs {
		if !s.pendingWrites[id] {
			filteredIDs = append(filteredIDs, id)
		}
		delete(s.pendingWrites, id)
	}

	s.mu.Unlock()

	if len(filteredIDs) == 0 && len(e.Files) == 0 {
		return
	}

	e.ThreadIDs = filteredIDs
	s.notify(e)
}

// markPendingWrite records a thread ID as being written by the service.
// The poller will skip this thread on its next scan.
func (s *Service) markPendingWrite(threadID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingWrites[threadID] = true
}

// signalShutdown closes the shutdown channel to signal service shutdown.
func (s *Service) signalShutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.shutdownCh:
		// Already closed
	default:
		close(s.shutdownCh)
	}
}
