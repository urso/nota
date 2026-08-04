package thread

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListThreads(t *testing.T) {
	dir := t.TempDir()

	// Create test threads
	threads := []*Thread{
		{ID: "l:0001000100010001", Status: "open", Goal: "review", Group: "pr-1"},
		{ID: "l:0002000200020002", Status: "resolved", Goal: "impl", Group: "pr-1"},
		{ID: "l:0003000300030003", Status: "open", Goal: "discuss", Tags: "auth,security"},
	}

	for _, th := range threads {
		th.Comments = []Comment{{
			ID:     "l:c001c001c001c001",
			Author: "test",
			Bodies: []Body{{Time: "2026-07-21T00:00:00Z", Content: "Test comment"}},
		}}
		filename := th.ID[2:] + ".xml"
		if err := WriteThread(filepath.Join(dir, filename), th); err != nil {
			t.Fatalf("failed to write thread: %v", err)
		}
	}

	tests := []struct {
		name   string
		filter ThreadFilter
		want   int
	}{
		{"no filter", ThreadFilter{}, 3},
		{"status open", ThreadFilter{Status: "open"}, 2},
		{"status resolved", ThreadFilter{Status: "resolved"}, 1},
		{"goal review", ThreadFilter{Goal: "review"}, 1},
		{"goal impl", ThreadFilter{Goal: "impl"}, 1},
		{"group pr-1", ThreadFilter{Group: "pr-1"}, 2},
		{"tag auth", ThreadFilter{Tag: "auth"}, 1},
		{"tag security", ThreadFilter{Tag: "security"}, 1},
		{"combined filters", ThreadFilter{Status: "open", Group: "pr-1"}, 1},
		{"no matches", ThreadFilter{Status: "wontfix"}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := ListThreads(dir, tt.filter)
			if err != nil {
				t.Fatalf("ListThreads failed: %v", err)
			}
			if len(results) != tt.want {
				t.Errorf("got %d threads, want %d", len(results), tt.want)
			}
		})
	}
}

func TestListThreadsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	results, err := ListThreads(dir, ThreadFilter{})
	if err != nil {
		t.Fatalf("ListThreads failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 threads, got %d", len(results))
	}
}

func TestListThreadsNonExistentDir(t *testing.T) {
	results, err := ListThreads("/nonexistent/path", ThreadFilter{})
	if err != nil {
		t.Fatalf("ListThreads should not error on nonexistent dir: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 threads, got %d", len(results))
	}
}

func TestListThreadsErrorsOnMalformedXML(t *testing.T) {
	dir := t.TempDir()

	// Write a valid thread
	th := &Thread{
		ID:     "l:0001000100010001",
		Status: "open",
		Comments: []Comment{{
			ID:     "l:c001c001c001c001",
			Author: "test",
			Bodies: []Body{{Time: "2026-07-21T00:00:00Z", Content: "Valid"}},
		}},
	}
	if err := WriteThread(filepath.Join(dir, "valid.xml"), th); err != nil {
		t.Fatal(err)
	}

	// Write malformed XML
	if err := os.WriteFile(filepath.Join(dir, "invalid.xml"), []byte("<broken"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := ListThreads(dir, ThreadFilter{})
	if err == nil {
		t.Fatal("expected error for malformed XML, got nil")
	}
	if !strings.Contains(err.Error(), "invalid.xml") {
		t.Errorf("error should mention the malformed file: %v", err)
	}
}

func TestFindThread(t *testing.T) {
	dir := t.TempDir()

	th := &Thread{
		ID:     "l:findme12345678ab",
		Status: "open",
		Comments: []Comment{{
			ID:     "l:c001c001c001c001",
			Author: "test",
			Bodies: []Body{{Time: "2026-07-21T00:00:00Z", Content: "Find me"}},
		}},
	}
	if err := WriteThread(filepath.Join(dir, "findme12345678ab.xml"), th); err != nil {
		t.Fatal(err)
	}

	t.Run("found", func(t *testing.T) {
		info, err := FindThread(dir, "l:findme12345678ab")
		if err != nil {
			t.Fatalf("FindThread failed: %v", err)
		}
		if info == nil {
			t.Fatal("expected to find thread")
		}
		if info.Thread.ID != "l:findme12345678ab" {
			t.Errorf("wrong thread ID: %s", info.Thread.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		info, err := FindThread(dir, "l:notfound1234567")
		if err != nil {
			t.Fatalf("FindThread failed: %v", err)
		}
		if info != nil {
			t.Error("expected nil for non-existent thread")
		}
	})
}

func TestThreadTitle(t *testing.T) {
	tests := []struct {
		name   string
		thread *Thread
		want   string
	}{
		{
			name:   "empty thread",
			thread: &Thread{},
			want:   "",
		},
		{
			name: "single line",
			thread: &Thread{
				Comments: []Comment{{
					Bodies: []Body{{Content: "Short title"}},
				}},
			},
			want: "Short title",
		},
		{
			name: "multi line takes first",
			thread: &Thread{
				Comments: []Comment{{
					Bodies: []Body{{Content: "First line\nSecond line\nThird line"}},
				}},
			},
			want: "First line",
		},
		{
			name: "truncates long titles",
			thread: &Thread{
				Comments: []Comment{{
					Bodies: []Body{{Content: "This is a very long title that exceeds sixty characters and should be truncated"}},
				}},
			},
			want: "This is a very long title that exceeds sixty characters a...",
		},
		{
			name: "exactly 60 chars no truncation",
			thread: &Thread{
				Comments: []Comment{{
					Bodies: []Body{{Content: "123456789012345678901234567890123456789012345678901234567890"}},
				}},
			},
			want: "123456789012345678901234567890123456789012345678901234567890",
		},
		{
			name: "61 chars truncates",
			thread: &Thread{
				Comments: []Comment{{
					Bodies: []Body{{Content: "1234567890123456789012345678901234567890123456789012345678901"}},
				}},
			},
			want: "123456789012345678901234567890123456789012345678901234567...",
		},
		{
			name: "trims whitespace",
			thread: &Thread{
				Comments: []Comment{{
					Bodies: []Body{{Content: "  Trimmed  \n"}},
				}},
			},
			want: "Trimmed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ThreadTitle(tt.thread)
			if got != tt.want {
				t.Errorf("ThreadTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidGoal(t *testing.T) {
	validGoals := []string{"review", "discuss", "impl", "explain", "refactor", "test", "doc", "propose", "critique"}
	for _, goal := range validGoals {
		if !ValidGoal(goal) {
			t.Errorf("ValidGoal(%q) = false, want true", goal)
		}
	}

	invalidGoals := []string{"", "invalid", "REVIEW", "Review", "todo", "fix", "review ", " review"}
	for _, goal := range invalidGoals {
		if ValidGoal(goal) {
			t.Errorf("ValidGoal(%q) = true, want false", goal)
		}
	}
}
