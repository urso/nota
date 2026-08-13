package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/urso/nota/pkg/thread"
)

func TestNew(t *testing.T) {
	dir := setupTestRepo(t)

	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Resolve symlinks for comparison (macOS /var -> /private/var)
	wantRoot, _ := filepath.EvalSymlinks(dir)
	gotRoot, _ := filepath.EvalSymlinks(svc.RepoRoot())
	if gotRoot != wantRoot {
		t.Errorf("RepoRoot() = %q, want %q", gotRoot, wantRoot)
	}

	wantNotaDir, _ := filepath.EvalSymlinks(filepath.Join(dir, ".nota"))
	gotNotaDir, _ := filepath.EvalSymlinks(svc.NotaDir())
	if gotNotaDir != wantNotaDir {
		t.Errorf("NotaDir() = %q, want %q", gotNotaDir, wantNotaDir)
	}
}

func TestNew_NoNotaDir(t *testing.T) {
	dir := t.TempDir()
	initGit(t, dir)

	_, err := New(dir)
	if err == nil {
		t.Fatal("New() expected error for missing .nota dir")
	}
}

func TestList_Empty(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	views, err := svc.List(Filter{})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(views) != 0 {
		t.Errorf("List() returned %d views, want 0", len(views))
	}
}

func TestCreateAndGet(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	view, err := svc.Create(CreateOpts{
		Message: "Test thread title\n\nBody content",
		Goal:    "review",
		Author:  "tester",
	})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if view.Title != "Test thread title" {
		t.Errorf("Title = %q, want %q", view.Title, "Test thread title")
	}
	if view.Thread.Status != "open" {
		t.Errorf("Status = %q, want %q", view.Thread.Status, "open")
	}
	if view.Thread.Goal != "review" {
		t.Errorf("Goal = %q, want %q", view.Thread.Goal, "review")
	}

	got, err := svc.Get(view.Thread.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Thread.ID != view.Thread.ID {
		t.Errorf("Get().ID = %q, want %q", got.Thread.ID, view.Thread.ID)
	}
}

func TestListWithFilter(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	_, err = svc.Create(CreateOpts{Message: "Review thread", Goal: "review", Author: "tester"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	_, err = svc.Create(CreateOpts{Message: "Impl thread", Goal: "impl", Author: "tester"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	views, err := svc.List(Filter{Goal: "review"})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(views) != 1 {
		t.Errorf("List(goal=review) returned %d views, want 1", len(views))
	}
	if len(views) > 0 && views[0].Thread.Goal != "review" {
		t.Errorf("List(goal=review)[0].Goal = %q, want review", views[0].Thread.Goal)
	}
}

func TestSetStatus(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	view, err := svc.Create(CreateOpts{Message: "Test", Author: "tester"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	updated, err := svc.SetStatus(view.Thread.ID, "resolved")
	if err != nil {
		t.Fatalf("SetStatus() error: %v", err)
	}
	if updated.Thread.Status != "resolved" {
		t.Errorf("Status = %q, want resolved", updated.Thread.Status)
	}
}

func TestAddComment(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	view, err := svc.Create(CreateOpts{Message: "Test", Author: "tester"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	updated, err := svc.AddComment(view.Thread.ID, AddCommentOpts{
		Message: "Reply comment",
		Author:  "tester",
	})
	if err != nil {
		t.Fatalf("AddComment() error: %v", err)
	}
	if len(updated.Thread.Comments) != 2 {
		t.Errorf("Comments count = %d, want 2", len(updated.Thread.Comments))
	}
}

func TestSubscribe(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ch := svc.Subscribe()
	defer svc.Unsubscribe(ch)

	_, err = svc.Create(CreateOpts{Message: "Test", Author: "tester"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	select {
	case e := <-ch:
		if len(e.ThreadIDs) != 1 {
			t.Errorf("Event has %d thread IDs, want 1", len(e.ThreadIDs))
		}
	default:
		t.Error("Expected event, got none")
	}
}

func TestNormalizePath(t *testing.T) {
	dir := setupTestRepo(t)
	svc, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// Use the actual repo root (may differ due to symlinks)
	repoRoot := svc.RepoRoot()

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{"relative", "foo/bar.go", "foo/bar.go", false},
		{"absolute inside", filepath.Join(repoRoot, "foo/bar.go"), "foo/bar.go", false},
		{"absolute outside", "/tmp/outside.go", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := svc.normalizePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("normalizePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("normalizePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFullTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"single line", "Title only", "Title only"},
		{"with body", "Title\n\nBody content", "Title"},
		{"with whitespace", "  Title  \n\nBody", "Title"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			th := &thread.Thread{
				Comments: []thread.Comment{{
					Bodies: []thread.Body{{Content: tt.content}},
				}},
			}
			if got := fullTitle(th); got != tt.want {
				t.Errorf("fullTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	initGit(t, dir)

	notaDir := filepath.Join(dir, ".nota")
	if err := os.MkdirAll(notaDir, 0o755); err != nil {
		t.Fatalf("creating .nota dir: %v", err)
	}

	return dir
}

func initGit(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
}
