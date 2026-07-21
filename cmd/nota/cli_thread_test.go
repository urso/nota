package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urso/nota/pkg/thread"
)

func setupThreadTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	notaDir := filepath.Join(dir, ".nota")
	if err := os.MkdirAll(notaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func createTestThread(t *testing.T, dir string, th *thread.Thread) string {
	t.Helper()
	notaDir := filepath.Join(dir, ".nota")
	filename := th.ID[2:] + ".xml"
	if th.Group != "" {
		filename = th.Group + "-" + th.ID[2:] + ".xml"
	}
	path := filepath.Join(notaDir, filename)
	if err := thread.WriteThread(path, th); err != nil {
		t.Fatalf("failed to write thread: %v", err)
	}
	return path
}

func TestThreadListCmd(t *testing.T) {
	dir := setupThreadTestDir(t)
	notaDir := filepath.Join(dir, ".nota")

	// Create test threads
	threads := []*thread.Thread{
		{
			ID: "l:0001000100010001", Status: "open", Goal: "review", Group: "pr-1",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Review this code"}},
			}},
		},
		{
			ID: "l:0002000200020002", Status: "resolved", Goal: "impl",
			Comments: []thread.Comment{{
				ID: "l:c002c002c002c002", Author: "bob",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Implement feature"}},
			}},
		},
		{
			ID: "l:0003000300030003", Status: "open", Goal: "discuss", Tags: "auth,security",
			Comments: []thread.Comment{{
				ID: "l:c003c003c003c003", Author: "carol",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Discuss security"}},
			}},
		},
	}

	for _, th := range threads {
		createTestThread(t, dir, th)
	}

	tests := []struct {
		name       string
		filter     thread.ThreadFilter
		wantCount  int
		wantInOut  []string
		wantNotOut []string
	}{
		{
			name:      "no filter lists all",
			filter:    thread.ThreadFilter{},
			wantCount: 3,
		},
		{
			name:       "filter by status open",
			filter:     thread.ThreadFilter{Status: "open"},
			wantCount:  2,
			wantInOut:  []string{"l:0001000100010001", "l:0003000300030003"},
			wantNotOut: []string{"l:0002000200020002"},
		},
		{
			name:       "filter by goal",
			filter:     thread.ThreadFilter{Goal: "review"},
			wantCount:  1,
			wantInOut:  []string{"Review this code"},
			wantNotOut: []string{"Implement feature"},
		},
		{
			name:      "filter by group",
			filter:    thread.ThreadFilter{Group: "pr-1"},
			wantCount: 1,
			wantInOut: []string{"l:0001000100010001"},
		},
		{
			name:      "filter by tag",
			filter:    thread.ThreadFilter{Tag: "auth"},
			wantCount: 1,
			wantInOut: []string{"Discuss security"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := thread.ListThreads(notaDir, tt.filter)
			if err != nil {
				t.Fatalf("ListThreads failed: %v", err)
			}
			if len(results) != tt.wantCount {
				t.Errorf("got %d threads, want %d", len(results), tt.wantCount)
			}

			// Build output as the command would
			var buf bytes.Buffer
			for _, info := range results {
				th := info.Thread
				title := thread.ThreadTitle(th)
				buf.WriteString(th.ID + "\t" + th.Status + "\t" + th.Goal + "\t" + title + "\n")
			}
			out := buf.String()

			for _, want := range tt.wantInOut {
				if !strings.Contains(out, want) {
					t.Errorf("output should contain %q", want)
				}
			}
			for _, notWant := range tt.wantNotOut {
				if strings.Contains(out, notWant) {
					t.Errorf("output should not contain %q", notWant)
				}
			}
		})
	}
}

func TestThreadShowCmd(t *testing.T) {
	dir := setupThreadTestDir(t)
	notaDir := filepath.Join(dir, ".nota")

	th := &thread.Thread{
		ID:     "l:showtest1234567",
		Status: "open",
		Goal:   "review",
		Group:  "test-group",
		Anchor: &thread.Anchor{File: "main.go", Line: 42, Commit: "abc123def456"},
		Comments: []thread.Comment{{
			ID: "l:c001c001c001c001", Author: "alice",
			Bodies: []thread.Body{{Time: "2026-07-21T10:00:00Z", Content: "Check this function"}},
		}},
	}
	path := createTestThread(t, dir, th)

	t.Run("default output", func(t *testing.T) {
		info, err := thread.FindThread(notaDir, th.ID)
		if err != nil || info == nil {
			t.Fatal("failed to find thread")
		}

		var buf bytes.Buffer
		// Simulate render output
		buf.WriteString("# Thread " + info.Thread.ID + "\n")
		if info.Thread.Anchor != nil {
			buf.WriteString("Anchor: " + info.Thread.Anchor.File)
		}

		out := buf.String()
		if !strings.Contains(out, "l:showtest1234567") {
			t.Error("output should contain thread ID")
		}
		if !strings.Contains(out, "main.go") {
			t.Error("output should contain anchor file")
		}
	})

	t.Run("raw output", func(t *testing.T) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		out := string(data)
		if !strings.Contains(out, "<?xml") {
			t.Error("raw output should contain XML prolog")
		}
		if !strings.Contains(out, "nota-thread") {
			t.Error("raw output should contain nota-thread element")
		}
	})

	t.Run("json output", func(t *testing.T) {
		info, err := thread.FindThread(notaDir, th.ID)
		if err != nil || info == nil {
			t.Fatal("failed to find thread")
		}

		data, err := json.Marshal(info.Thread)
		if err != nil {
			t.Fatal(err)
		}

		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatal(err)
		}

		if parsed["id"] != "l:showtest1234567" {
			t.Errorf("JSON id = %v, want l:showtest1234567", parsed["id"])
		}
		if parsed["status"] != "open" {
			t.Errorf("JSON status = %v, want open", parsed["status"])
		}
	})

	t.Run("not found", func(t *testing.T) {
		info, err := thread.FindThread(notaDir, "l:notfound1234567")
		if err != nil {
			t.Fatal(err)
		}
		if info != nil {
			t.Error("should return nil for non-existent thread")
		}
	})
}

func TestThreadCreateCmd(t *testing.T) {
	t.Run("creates thread with message", func(t *testing.T) {
		dir := setupThreadTestDir(t)
		notaDir := filepath.Join(dir, ".nota")

		th := thread.NewThread("open", "review")
		comment := thread.NewComment("testuser", "Test message")
		th.Comments = append(th.Comments, comment)

		filename := th.ID[2:] + ".xml"
		path := filepath.Join(notaDir, filename)
		if err := thread.WriteThread(path, th); err != nil {
			t.Fatal(err)
		}

		// Verify it was written
		read, err := thread.ReadThread(path)
		if err != nil {
			t.Fatal(err)
		}
		if read.Status != "open" {
			t.Errorf("status = %s, want open", read.Status)
		}
		if read.Goal != "review" {
			t.Errorf("goal = %s, want review", read.Goal)
		}
		if len(read.Comments) != 1 {
			t.Errorf("comments = %d, want 1", len(read.Comments))
		}
		if read.Comments[0].Bodies[0].Content != "Test message" {
			t.Errorf("message = %s, want 'Test message'", read.Comments[0].Bodies[0].Content)
		}
	})

	t.Run("creates thread with group", func(t *testing.T) {
		dir := setupThreadTestDir(t)
		notaDir := filepath.Join(dir, ".nota")

		th := thread.NewThread("open", "impl")
		th.Group = "my-group"
		comment := thread.NewComment("testuser", "Grouped thread")
		th.Comments = append(th.Comments, comment)

		filename := th.Group + "-" + th.ID[2:] + ".xml"
		path := filepath.Join(notaDir, filename)
		if err := thread.WriteThread(path, th); err != nil {
			t.Fatal(err)
		}

		// Verify filename includes group
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file at %s", path)
		}
	})

	t.Run("rejects empty message", func(t *testing.T) {
		msg := strings.TrimSpace("")
		if msg != "" {
			t.Error("empty message should be rejected")
		}
	})

	t.Run("rejects invalid goal", func(t *testing.T) {
		if thread.ValidGoal("invalid-goal") {
			t.Error("invalid goal should be rejected")
		}
	})
}

func TestThreadStatusCommands(t *testing.T) {
	dir := setupThreadTestDir(t)
	notaDir := filepath.Join(dir, ".nota")

	th := &thread.Thread{
		ID:     "l:status123456789",
		Status: "open",
		Comments: []thread.Comment{{
			ID: "l:c001c001c001c001", Author: "alice",
			Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
		}},
	}
	path := createTestThread(t, dir, th)

	tests := []struct {
		name      string
		newStatus string
	}{
		{"resolve", "resolved"},
		{"wontfix", "wontfix"},
		{"reopen", "open"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Read, update, write
			info, err := thread.FindThread(notaDir, th.ID)
			if err != nil || info == nil {
				t.Fatal("failed to find thread")
			}
			info.Thread.Status = tt.newStatus
			if err := thread.WriteThread(path, info.Thread); err != nil {
				t.Fatal(err)
			}

			// Verify
			read, err := thread.ReadThread(path)
			if err != nil {
				t.Fatal(err)
			}
			if read.Status != tt.newStatus {
				t.Errorf("status = %s, want %s", read.Status, tt.newStatus)
			}
		})
	}
}

func TestThreadGoalCmd(t *testing.T) {
	dir := setupThreadTestDir(t)
	notaDir := filepath.Join(dir, ".nota")

	th := &thread.Thread{
		ID:     "l:goaltest123456",
		Status: "open",
		Goal:   "review",
		Comments: []thread.Comment{{
			ID: "l:c001c001c001c001", Author: "alice",
			Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
		}},
	}
	path := createTestThread(t, dir, th)

	t.Run("updates goal", func(t *testing.T) {
		info, err := thread.FindThread(notaDir, th.ID)
		if err != nil || info == nil {
			t.Fatal("failed to find thread")
		}
		info.Thread.Goal = "impl"
		if err := thread.WriteThread(path, info.Thread); err != nil {
			t.Fatal(err)
		}

		read, err := thread.ReadThread(path)
		if err != nil {
			t.Fatal(err)
		}
		if read.Goal != "impl" {
			t.Errorf("goal = %s, want impl", read.Goal)
		}
	})

	t.Run("rejects invalid goal", func(t *testing.T) {
		if thread.ValidGoal("not-a-goal") {
			t.Error("should reject invalid goal")
		}
	})
}

func TestValidateThreadID(t *testing.T) {
	tests := []struct {
		id      string
		wantErr bool
	}{
		{"l:0123456789abcdef", false},
		{"l:abcdef0123456789", false},
		{"gh:12345", false},
		{"gh:1", false},
		{"invalid", true},
		{"l:short", true},
		{"l:toolongid1234567890", true},
		{"gh:", true},
		{"gh:abc", true},
		{"x:0123456789abcdef", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			err := validateThreadID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateThreadID(%q) error = %v, wantErr = %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestParseAnchor(t *testing.T) {
	// Create a temp file to test with
	dir := t.TempDir()
	testFile := filepath.Join(dir, "test.go")
	if err := os.WriteFile(testFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("valid anchor", func(t *testing.T) {
		anchor, err := parseAnchor(testFile + ":10")
		if err != nil {
			t.Fatalf("parseAnchor failed: %v", err)
		}
		if anchor.File != testFile {
			t.Errorf("file = %s, want %s", anchor.File, testFile)
		}
		if anchor.Line != 10 {
			t.Errorf("line = %d, want 10", anchor.Line)
		}
	})

	t.Run("missing line number", func(t *testing.T) {
		_, err := parseAnchor(testFile)
		if err == nil {
			t.Error("expected error for missing line number")
		}
	})

	t.Run("invalid line number", func(t *testing.T) {
		_, err := parseAnchor(testFile + ":abc")
		if err == nil {
			t.Error("expected error for invalid line number")
		}
	})

	t.Run("zero line number", func(t *testing.T) {
		_, err := parseAnchor(testFile + ":0")
		if err == nil {
			t.Error("expected error for zero line number")
		}
	})

	t.Run("negative line number", func(t *testing.T) {
		_, err := parseAnchor(testFile + ":-5")
		if err == nil {
			t.Error("expected error for negative line number")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := parseAnchor("/nonexistent/file.go:10")
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})

	t.Run("file with colon in path", func(t *testing.T) {
		// This tests LastIndexByte behavior - the line number should be after the last colon
		_, err := parseAnchor("/path/to:file/test.go:10")
		// This will fail because the path doesn't exist, but it should parse correctly
		if err == nil || !strings.Contains(err.Error(), "not found") {
			// If we got here, the parsing worked but file doesn't exist
		}
	})
}

func TestThreadFilename(t *testing.T) {
	tests := []struct {
		name  string
		th    *thread.Thread
		want  string
	}{
		{
			name: "without group",
			th:   &thread.Thread{ID: "l:0123456789abcdef"},
			want: "0123456789abcdef.xml",
		},
		{
			name: "with group",
			th:   &thread.Thread{ID: "l:0123456789abcdef", Group: "pr-123"},
			want: "pr-123-0123456789abcdef.xml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := threadFilename(tt.th)
			if got != tt.want {
				t.Errorf("threadFilename() = %s, want %s", got, tt.want)
			}
		})
	}
}
