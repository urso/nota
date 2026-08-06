package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urso/nota/pkg/github"
	"github.com/urso/nota/pkg/github/ghapi"
	"github.com/urso/nota/pkg/thread"
)

func TestSyncPullCmd_Summary(t *testing.T) {
	dir := t.TempDir()
	notaDir := filepath.Join(dir, ".nota")
	os.MkdirAll(notaDir, 0o755)

	threads := []ghapi.ReviewThread{
		{
			ID:          "PRRT_1",
			IsResolved:  false,
			SubjectType: "LINE",
			Path:        "main.go",
			Line:        intPtr(10),
			CommitID:    "abc123",
			Comments: []ghapi.ReviewComment{
				{ID: "PRRC_1", FullDatabaseID: "100", Author: "alice", Body: "Fix", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"},
			},
		},
		{
			ID:          "PRRT_2",
			IsResolved:  true,
			SubjectType: "LINE",
			Path:        "other.go",
			Line:        intPtr(20),
			CommitID:    "def456",
			Comments: []ghapi.ReviewComment{
				{ID: "PRRC_2", FullDatabaseID: "101", Author: "bob", Body: "LGTM", CreatedAt: "2026-01-02T00:00:00Z", UpdatedAt: "2026-01-02T00:00:00Z"},
			},
		},
	}

	reviews := []ghapi.Review{
		{ID: 1, NodeID: "PRR_1", Body: "Approved", State: "APPROVED", SubmittedAt: "2026-01-03T00:00:00Z", User: &ghapi.User{Login: "carol"}},
	}

	prInfo := ghapi.PRInfo{ID: "PR_123", Number: 123}
	opts := github.PullOptions{
		Group:        "pr-123",
		ResolvedRepo: ghapi.Repo{Owner: "owner", Name: "repo"},
	}

	result, err := github.Pull(notaDir, threads, reviews, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}

	var buf bytes.Buffer
	printPullSummary(&buf, result, 123)
	output := buf.String()

	wantPhrases := []string{
		"PR #123",
		"Inline threads: 2",
		"1 resolved",
		"1 open",
		"Conversation:   1 thread",
		"3 new",
		"Group:          pr-123",
		"nota thread list --group=pr-123",
	}

	for _, phrase := range wantPhrases {
		if !strings.Contains(output, phrase) {
			t.Errorf("output missing %q\n\nGot:\n%s", phrase, output)
		}
	}
}

func TestSyncPullCmd_AlreadyUpToDate(t *testing.T) {
	dir := t.TempDir()
	notaDir := filepath.Join(dir, ".nota")
	os.MkdirAll(notaDir, 0o755)

	threads := []ghapi.ReviewThread{
		{
			ID:          "PRRT_1",
			IsResolved:  false,
			SubjectType: "LINE",
			Path:        "main.go",
			Line:        intPtr(10),
			CommitID:    "abc123",
			Comments: []ghapi.ReviewComment{
				{ID: "PRRC_1", FullDatabaseID: "100", Author: "alice", Body: "Fix", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"},
			},
		},
	}

	prInfo := ghapi.PRInfo{ID: "PR_123", Number: 123}
	opts := github.PullOptions{
		Group:        "pr-123",
		ResolvedRepo: ghapi.Repo{Owner: "owner", Name: "repo"},
	}

	github.Pull(notaDir, threads, nil, nil, prInfo, opts)

	result, _ := github.Pull(notaDir, threads, nil, nil, prInfo, opts)

	var buf bytes.Buffer
	printPullSummary(&buf, result, 123)
	output := buf.String()

	if !strings.Contains(output, "already up to date") {
		t.Errorf("expected 'already up to date' message, got:\n%s", output)
	}
}

func TestSyncPullCmd_ThreadsCreated(t *testing.T) {
	dir := t.TempDir()
	notaDir := filepath.Join(dir, ".nota")
	os.MkdirAll(notaDir, 0o755)

	threads := []ghapi.ReviewThread{
		{
			ID:          "PRRT_1",
			IsResolved:  false,
			SubjectType: "LINE",
			Path:        "main.go",
			Line:        intPtr(10),
			CommitID:    "abc123",
			Comments: []ghapi.ReviewComment{
				{ID: "PRRC_1", FullDatabaseID: "100", Author: "alice", Body: "Fix this", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"},
			},
		},
	}

	prInfo := ghapi.PRInfo{ID: "PR_123", Number: 123}
	opts := github.PullOptions{
		Group:        "pr-123",
		ResolvedRepo: ghapi.Repo{Owner: "owner", Name: "repo"},
	}

	_, err := github.Pull(notaDir, threads, nil, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(notaDir, "*.xml"))
	if len(files) != 1 {
		t.Fatalf("expected 1 thread file, got %d", len(files))
	}

	th, err := thread.ReadThread(files[0])
	if err != nil {
		t.Fatalf("reading thread: %v", err)
	}

	if th.Goal != "review" {
		t.Errorf("Goal = %q, want %q", th.Goal, "review")
	}

	if th.Sync == nil {
		t.Fatal("Sync config should be set")
	}

	if th.Sync.Provider != "github" {
		t.Errorf("Sync.Provider = %q, want %q", th.Sync.Provider, "github")
	}

	if th.Sync.ThreadID != "PRRT_1" {
		t.Errorf("Sync.ThreadID = %q, want %q", th.Sync.ThreadID, "PRRT_1")
	}

	if len(th.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(th.Comments))
	}

	if th.Comments[0].Author != "github:alice" {
		t.Errorf("Author = %q, want %q", th.Comments[0].Author, "github:alice")
	}

	if th.Comments[0].SyncStatus != "pulled" {
		t.Errorf("SyncStatus = %q, want %q", th.Comments[0].SyncStatus, "pulled")
	}
}

func TestSyncPullCmd_GroupDefault(t *testing.T) {
	dir := t.TempDir()
	notaDir := filepath.Join(dir, ".nota")
	os.MkdirAll(notaDir, 0o755)

	threads := []ghapi.ReviewThread{
		{
			ID:          "PRRT_1",
			SubjectType: "LINE",
			Path:        "main.go",
			Line:        intPtr(10),
			Comments: []ghapi.ReviewComment{
				{ID: "PRRC_1", FullDatabaseID: "100", Author: "a", Body: "b", CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"},
			},
		},
	}

	prInfo := ghapi.PRInfo{ID: "PR_456", Number: 456}
	opts := github.PullOptions{
		Group:        "pr-456",
		ResolvedRepo: ghapi.Repo{Owner: "owner", Name: "repo"},
	}

	_, err := github.Pull(notaDir, threads, nil, nil, prInfo, opts)
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}

	files, _ := filepath.Glob(filepath.Join(notaDir, "pr-456-*.xml"))
	if len(files) != 1 {
		t.Errorf("expected file with pr-456 prefix, got %d matches", len(files))
	}
}

func intPtr(i int) *int {
	return &i
}
