package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/urso/nota/pkg/thread"
)

func TestPollerDetectsNewFile(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.StartPoller(ctx, 50*time.Millisecond)
	defer svc.StopPoller()

	ch := svc.Subscribe()
	defer svc.Unsubscribe(ch)

	// Create a thread file directly (simulating external write)
	th := &thread.Thread{
		ID:     "l:test123",
		Number: 1,
		Status: "open",
		Comments: []thread.Comment{{
			ID:     "c1",
			Author: "test",
			Bodies: []thread.Body{{Time: "2024-01-01T00:00:00Z", Content: "Test"}},
		}},
	}
	path := filepath.Join(svc.NotaDir(), "0001-test123.xml")
	if err := thread.WriteThread(path, th); err != nil {
		t.Fatalf("WriteThread error: %v", err)
	}

	select {
	case e := <-ch:
		if len(e.ThreadIDs) != 1 || e.ThreadIDs[0] != "l:test123" {
			t.Errorf("Expected thread ID l:test123, got %v", e.ThreadIDs)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Timeout waiting for change event")
	}
}

func TestPollerDetectsModifiedFile(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Create initial thread
	th := &thread.Thread{
		ID:     "l:mod123",
		Number: 1,
		Status: "open",
		Comments: []thread.Comment{{
			ID:     "c1",
			Author: "test",
			Bodies: []thread.Body{{Time: "2024-01-01T00:00:00Z", Content: "Initial"}},
		}},
	}
	path := filepath.Join(svc.NotaDir(), "0001-mod123.xml")
	if err := thread.WriteThread(path, th); err != nil {
		t.Fatalf("WriteThread error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.StartPoller(ctx, 50*time.Millisecond)
	defer svc.StopPoller()

	ch := svc.Subscribe()
	defer svc.Unsubscribe(ch)

	// Wait for initial scan
	time.Sleep(100 * time.Millisecond)

	// Drain any initial events
	select {
	case <-ch:
	default:
	}

	// Modify the file
	time.Sleep(10 * time.Millisecond) // Ensure mtime changes
	th.Status = "resolved"
	if err := thread.WriteThread(path, th); err != nil {
		t.Fatalf("WriteThread error: %v", err)
	}

	select {
	case e := <-ch:
		if len(e.ThreadIDs) != 1 || e.ThreadIDs[0] != "l:mod123" {
			t.Errorf("Expected thread ID l:mod123, got %v", e.ThreadIDs)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Timeout waiting for change event")
	}
}

func TestPollerServiceWriteNoDuplicate(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.StartPoller(ctx, 50*time.Millisecond)
	defer svc.StopPoller()

	ch := svc.Subscribe()
	defer svc.Unsubscribe(ch)

	// Create via service (should get immediate notification)
	view, err := svc.Create(CreateOpts{Message: "Service write", Author: "tester"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Should receive the immediate notification
	select {
	case e := <-ch:
		if len(e.ThreadIDs) != 1 || e.ThreadIDs[0] != view.Thread.ID {
			t.Errorf("Expected thread ID %s, got %v", view.Thread.ID, e.ThreadIDs)
		}
	default:
		t.Error("Expected immediate notification")
	}

	// Wait for poller cycle - should NOT get duplicate
	time.Sleep(150 * time.Millisecond)

	select {
	case e := <-ch:
		t.Errorf("Unexpected duplicate event: %v", e)
	default:
		// Good - no duplicate
	}
}

func TestPollerPauseResume(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.StartPoller(ctx, 50*time.Millisecond)
	defer svc.StopPoller()

	// Subscribe then unsubscribe to pause
	ch := svc.Subscribe()
	svc.Unsubscribe(ch)

	if svc.SubscriberCount() != 0 {
		t.Errorf("Expected 0 subscribers, got %d", svc.SubscriberCount())
	}

	// Create a thread while paused
	th := &thread.Thread{
		ID:     "l:paused123",
		Number: 1,
		Status: "open",
		Comments: []thread.Comment{{
			ID:     "c1",
			Author: "test",
			Bodies: []thread.Body{{Time: "2024-01-01T00:00:00Z", Content: "While paused"}},
		}},
	}
	path := filepath.Join(svc.NotaDir(), "0001-paused123.xml")
	if err := thread.WriteThread(path, th); err != nil {
		t.Fatalf("WriteThread error: %v", err)
	}

	// Re-subscribe - should get the change
	ch2 := svc.Subscribe()
	defer svc.Unsubscribe(ch2)

	select {
	case e := <-ch2:
		found := false
		for _, id := range e.ThreadIDs {
			if id == "l:paused123" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected thread ID l:paused123 in %v", e.ThreadIDs)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Timeout waiting for accumulated change event")
	}
}

func TestPollerDirectoryDisappears(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.StartPoller(ctx, 50*time.Millisecond)
	defer svc.StopPoller()

	ch := svc.Subscribe()
	defer svc.Unsubscribe(ch)

	// Remove .nota directory
	if err := os.RemoveAll(svc.NotaDir()); err != nil {
		t.Fatalf("RemoveAll error: %v", err)
	}

	// Should signal shutdown
	select {
	case <-svc.ShutdownCh():
		// Good
	case <-time.After(500 * time.Millisecond):
		t.Error("Expected shutdown signal")
	}
}

func TestPollerContextCancellation(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	svc.StartPoller(ctx, 50*time.Millisecond)

	ch := svc.Subscribe()
	defer svc.Unsubscribe(ch)

	// Cancel context
	cancel()

	// Give poller time to stop
	time.Sleep(100 * time.Millisecond)

	// Create a thread - should not trigger event since poller stopped
	th := &thread.Thread{
		ID:     "l:aftercancel",
		Number: 1,
		Status: "open",
		Comments: []thread.Comment{{
			ID:     "c1",
			Author: "test",
			Bodies: []thread.Body{{Time: "2024-01-01T00:00:00Z", Content: "After cancel"}},
		}},
	}
	path := filepath.Join(svc.NotaDir(), "0001-aftercancel.xml")
	if err := thread.WriteThread(path, th); err != nil {
		t.Fatalf("WriteThread error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	select {
	case e := <-ch:
		t.Errorf("Unexpected event after cancel: %v", e)
	default:
		// Good - no event
	}
}

func TestSnapshotDiff(t *testing.T) {
	p := &Poller{}

	old := map[string]fileSnapshot{
		"/a.xml": {size: 100},
		"/b.xml": {size: 200},
		"/c.xml": {size: 300},
	}

	current := map[string]fileSnapshot{
		"/a.xml": {size: 100},      // unchanged
		"/b.xml": {size: 250},      // modified
		"/d.xml": {size: 400},      // added
		// /c.xml removed
	}

	added, removed, modified := p.diff(old, current)

	if len(added) != 1 || added[0] != "/d.xml" {
		t.Errorf("Added = %v, want [/d.xml]", added)
	}
	if len(removed) != 1 || removed[0] != "/c.xml" {
		t.Errorf("Removed = %v, want [/c.xml]", removed)
	}
	if len(modified) != 1 || modified[0] != "/b.xml" {
		t.Errorf("Modified = %v, want [/b.xml]", modified)
	}
}
