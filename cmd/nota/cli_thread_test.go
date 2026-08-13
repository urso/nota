package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

func setupThreadTestDirWithGit(t *testing.T) string {
	t.Helper()
	dir := setupThreadTestDir(t)
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
	return dir
}

func createTestThread(t *testing.T, dir string, th *thread.Thread) string {
	t.Helper()
	notaDir := filepath.Join(dir, ".nota")
	filename := thread.Filename(th)
	path := filepath.Join(notaDir, filename)
	if err := thread.WriteThread(path, th); err != nil {
		t.Fatalf("failed to write thread: %v", err)
	}
	return path
}

func TestThreadListCmd(t *testing.T) {
	dir := setupThreadTestDir(t)

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
		cmd        ThreadListCmd
		wantCount  int
		wantInOut  []string
		wantNotOut []string
	}{
		{
			name:      "no filter lists all",
			cmd:       ThreadListCmd{},
			wantCount: 3,
		},
		{
			name:       "filter by status open",
			cmd:        ThreadListCmd{Status: "open"},
			wantCount:  2,
			wantInOut:  []string{"l:0001000100010001", "l:0003000300030003"},
			wantNotOut: []string{"l:0002000200020002"},
		},
		{
			name:       "filter by goal",
			cmd:        ThreadListCmd{Goal: "review"},
			wantCount:  1,
			wantInOut:  []string{"Review this code"},
			wantNotOut: []string{"Implement feature"},
		},
		{
			name:      "filter by group",
			cmd:       ThreadListCmd{Group: "pr-1"},
			wantCount: 1,
			wantInOut: []string{"l:0001000100010001"},
		},
		{
			name:      "filter by tag",
			cmd:       ThreadListCmd{Tag: "auth"},
			wantCount: 1,
			wantInOut: []string{"Discuss security"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := tt.cmd.run(&buf, dir)
			if err != nil {
				t.Fatalf("run() failed: %v", err)
			}

			out := buf.String()
			lines := strings.Split(strings.TrimSpace(out), "\n")
			if len(lines) == 1 && lines[0] == "" {
				lines = nil
			}

			if len(lines) != tt.wantCount {
				t.Errorf("got %d threads, want %d\noutput: %s", len(lines), tt.wantCount, out)
			}

			for _, want := range tt.wantInOut {
				if !strings.Contains(out, want) {
					t.Errorf("output should contain %q\ngot: %s", want, out)
				}
			}
			for _, notWant := range tt.wantNotOut {
				if strings.Contains(out, notWant) {
					t.Errorf("output should not contain %q\ngot: %s", notWant, out)
				}
			}
		})
	}
}

func TestThreadListCmd_JSON(t *testing.T) {
	dir := setupThreadTestDirWithGit(t)

	threads := []*thread.Thread{
		{
			ID: "l:json000100010001", Number: 1, Status: "open", Goal: "review",
			Tags:    "bug,urgent",
			Anchors: []thread.Anchor{{File: "main.go", Line: 10}},
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "First thread"}},
			}},
		},
		{
			ID: "l:json000200020002", Number: 2, Status: "open", Goal: "impl",
			Anchors: []thread.Anchor{{File: "other.go", Line: 20}},
			Comments: []thread.Comment{{
				ID: "l:c002c002c002c002", Author: "bob",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Second thread"}},
			}},
		},
	}

	for _, th := range threads {
		createTestThread(t, dir, th)
	}

	t.Run("json output", func(t *testing.T) {
		cmd := ThreadListCmd{JSON: true}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		var result []map[string]any
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON output: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 threads, got %d", len(result))
		}
		if result[0]["id"] != "l:json000100010001" {
			t.Errorf("first thread id = %v, want l:json000100010001", result[0]["id"])
		}
		if result[1]["id"] != "l:json000200020002" {
			t.Errorf("second thread id = %v, want l:json000200020002", result[1]["id"])
		}
		if _, ok := result[0]["tags"].([]any); !ok {
			t.Errorf("tags should be array, got %T", result[0]["tags"])
		}
		if result[0]["title"] != "First thread" {
			t.Errorf("first thread title = %v, want First thread", result[0]["title"])
		}
	})

	t.Run("file filter", func(t *testing.T) {
		cmd := ThreadListCmd{File: "main.go"}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		out := buf.String()
		if !strings.Contains(out, "l:json000100010001") {
			t.Errorf("output should contain thread anchored to main.go\ngot: %s", out)
		}
		if strings.Contains(out, "l:json000200020002") {
			t.Errorf("output should not contain thread anchored to other.go\ngot: %s", out)
		}
	})
}

func TestThreadShowCmd(t *testing.T) {
	dir := setupThreadTestDir(t)

	th := &thread.Thread{
		ID:      "l:showtest12345678",
		Status:  "open",
		Goal:    "review",
		Group:   "test-group",
		Anchors: []thread.Anchor{{File: "main.go", Line: 42, Commit: "abc123def456"}},
		Comments: []thread.Comment{{
			ID: "l:c001c001c001c001", Author: "alice",
			Bodies: []thread.Body{{Time: "2026-07-21T10:00:00Z", Content: "Check this function"}},
		}},
	}
	createTestThread(t, dir, th)

	t.Run("default output", func(t *testing.T) {
		cmd := ThreadShowCmd{ID: th.ID}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		out := buf.String()
		if !strings.Contains(out, "l:showtest12345678") {
			t.Error("output should contain thread ID")
		}
		if !strings.Contains(out, "main.go:42") {
			t.Error("output should contain anchor")
		}
		if !strings.Contains(out, "alice") {
			t.Error("output should contain author")
		}
		if !strings.Contains(out, "Check this function") {
			t.Error("output should contain comment body")
		}
	})

	t.Run("raw output", func(t *testing.T) {
		cmd := ThreadShowCmd{ID: th.ID, Raw: true}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		out := buf.String()
		if !strings.Contains(out, "<?xml") {
			t.Error("raw output should contain XML prolog")
		}
		if !strings.Contains(out, "nota-thread") {
			t.Error("raw output should contain nota-thread element")
		}
	})

	t.Run("json output", func(t *testing.T) {
		// JSON output uses service layer which requires git
		gitDir := setupThreadTestDirWithGit(t)
		jsonThread := &thread.Thread{
			ID:      "l:jsontest12345678",
			Number:  1,
			Status:  "open",
			Goal:    "review",
			Tags:    "tag1,tag2",
			Anchors: []thread.Anchor{{File: "main.go", Line: 42}},
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T10:00:00Z", Content: "Test title\n\nBody"}},
			}},
		}
		createTestThread(t, gitDir, jsonThread)

		cmd := ThreadShowCmd{ID: jsonThread.ID, JSON: true}
		var buf bytes.Buffer
		err := cmd.run(&buf, gitDir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		out := buf.String()
		if !strings.Contains(out, `"id": "l:jsontest12345678"`) {
			t.Errorf("JSON should contain id field\ngot: %s", out)
		}
		if !strings.Contains(out, `"status": "open"`) {
			t.Errorf("JSON should contain status field\ngot: %s", out)
		}
		if !strings.Contains(out, `"title": "Test title"`) {
			t.Errorf("JSON should contain title field\ngot: %s", out)
		}
		if !strings.Contains(out, `"tags": [`) {
			t.Errorf("JSON should contain tags as array\ngot: %s", out)
		}
	})

	t.Run("not found", func(t *testing.T) {
		cmd := ThreadShowCmd{ID: "l:notfound12345678"}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err == nil {
			t.Error("expected error for non-existent thread")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error should mention 'not found', got: %v", err)
		}
	})
}

func TestThreadShowFileAnchor(t *testing.T) {
	dir := setupThreadTestDir(t)

	th := &thread.Thread{
		ID:          "l:fileanchor123456",
		Status:      "open",
		Goal:        "review",
		FileAnchors: []thread.FileAnchor{{File: "handlers/auth.go", Commit: "abc123def456"}},
		Comments: []thread.Comment{{
			ID: "l:c001c001c001c001", Author: "github:alice",
			Bodies: []thread.Body{{Time: "2026-07-21T10:00:00Z", Content: "This file needs a package doc"}},
		}},
	}
	createTestThread(t, dir, th)

	cmd := ThreadShowCmd{ID: th.ID, Authors: authorsAll}
	var buf bytes.Buffer
	if err := cmd.run(&buf, dir); err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	out := buf.String()
	// A file anchor renders under the same "Anchor:" label; the absent line
	// number is what distinguishes it.
	if !strings.Contains(out, "Anchor: handlers/auth.go @ abc123d") {
		t.Errorf("expected file anchor without a line number, got:\n%s", out)
	}
	if strings.Contains(out, "handlers/auth.go:") {
		t.Errorf("file anchor must not render a line number, got:\n%s", out)
	}
}

func TestThreadShowAuthorsFilter(t *testing.T) {
	dir := setupThreadTestDir(t)

	th := &thread.Thread{
		ID:     "l:authorfilter1234",
		Status: "open",
		Goal:   "review",
		Comments: []thread.Comment{
			{ID: "l:c001c001c001c001", Author: "github:alice", Bodies: []thread.Body{{Time: "t1", Content: "human one"}}},
			{ID: "l:c002c002c002c002", Author: "github:prow[bot]", Bodies: []thread.Body{{Time: "t2", Content: "bot one"}}},
			{ID: "l:c003c003c003c003", Author: "github:bob", Bodies: []thread.Body{{Time: "t3", Content: "human two"}}},
		},
	}
	createTestThread(t, dir, th)

	tests := []struct {
		name       string
		authors    string
		wantInOut  []string
		wantNotOut []string
	}{
		{
			name:      "default all shows bots",
			authors:   authorsAll,
			wantInOut: []string{"human one", "bot one", "human two"},
		},
		{
			name:       "humans excludes bots",
			authors:    authorsHumans,
			wantInOut:  []string{"human one", "human two"},
			wantNotOut: []string{"bot one"},
		},
		{
			name:       "bots excludes humans",
			authors:    authorsBots,
			wantInOut:  []string{"bot one"},
			wantNotOut: []string{"human one", "human two"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := ThreadShowCmd{ID: th.ID, Authors: tt.authors}
			var buf bytes.Buffer
			if err := cmd.run(&buf, dir); err != nil {
				t.Fatalf("run() failed: %v", err)
			}

			out := buf.String()
			for _, want := range tt.wantInOut {
				if !strings.Contains(out, want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, out)
				}
			}
			for _, notWant := range tt.wantNotOut {
				if strings.Contains(out, notWant) {
					t.Errorf("expected output not to contain %q, got:\n%s", notWant, out)
				}
			}
		})
	}
}

func TestThreadShowLimit(t *testing.T) {
	dir := setupThreadTestDir(t)

	th := &thread.Thread{
		ID:     "l:limittest123456a",
		Status: "open",
		Goal:   "review",
		Comments: []thread.Comment{
			{ID: "l:c001c001c001c001", Author: "alice", Bodies: []thread.Body{{Time: "t1", Content: "the review concern"}}},
			{ID: "l:c002c002c002c002", Author: "bob", Bodies: []thread.Body{{Time: "t2", Content: "second reply"}}},
			{ID: "l:c003c003c003c003", Author: "carol", Bodies: []thread.Body{{Time: "t3", Content: "third reply"}}},
			{ID: "l:c004c004c004c004", Author: "dave", Bodies: []thread.Body{{Time: "t4", Content: "fourth reply"}}},
		},
	}
	createTestThread(t, dir, th)

	t.Run("unbounded by default", func(t *testing.T) {
		cmd := ThreadShowCmd{ID: th.ID, Authors: authorsAll}
		var buf bytes.Buffer
		if err := cmd.run(&buf, dir); err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		out := buf.String()
		for _, want := range []string{"the review concern", "second reply", "third reply", "fourth reply"} {
			if !strings.Contains(out, want) {
				t.Errorf("expected output to contain %q, got:\n%s", want, out)
			}
		}
		if strings.Contains(out, "omitted") {
			t.Errorf("unbounded output should not elide, got:\n%s", out)
		}
	})

	t.Run("limit keeps the opening comment and marks the gap", func(t *testing.T) {
		cmd := ThreadShowCmd{ID: th.ID, Authors: authorsAll, Limit: 2}
		var buf bytes.Buffer
		if err := cmd.run(&buf, dir); err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		out := buf.String()
		// The first comment is the review concern; everything after responds
		// to it, so it survives the limit.
		if !strings.Contains(out, "the review concern") {
			t.Errorf("expected the opening comment to always be shown, got:\n%s", out)
		}
		if !strings.Contains(out, "third reply") || !strings.Contains(out, "fourth reply") {
			t.Errorf("expected the last 2 comments, got:\n%s", out)
		}
		if strings.Contains(out, "second reply") {
			t.Errorf("expected the second comment to be elided, got:\n%s", out)
		}
		// Silent truncation is indistinguishable from a short thread.
		if !strings.Contains(out, "... 1 comments omitted ...") {
			t.Errorf("expected an elision marker for 1 omitted comment, got:\n%s", out)
		}
	})

	t.Run("limit covering the whole thread does not elide", func(t *testing.T) {
		cmd := ThreadShowCmd{ID: th.ID, Authors: authorsAll, Limit: 4}
		var buf bytes.Buffer
		if err := cmd.run(&buf, dir); err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		if out := buf.String(); strings.Contains(out, "omitted") {
			t.Errorf("limit >= comment count should not elide, got:\n%s", out)
		}
	})

	t.Run("limit applies after the author filter", func(t *testing.T) {
		botThread := &thread.Thread{
			ID:     "l:limitbots1234567",
			Status: "open",
			Comments: []thread.Comment{
				{ID: "l:d001d001d001d001", Author: "alice", Bodies: []thread.Body{{Time: "t1", Content: "opening concern"}}},
				{ID: "l:d002d002d002d002", Author: "prow[bot]", Bodies: []thread.Body{{Time: "t2", Content: "ci noise"}}},
				{ID: "l:d003d003d003d003", Author: "bob", Bodies: []thread.Body{{Time: "t3", Content: "human reply one"}}},
				{ID: "l:d004d004d004d004", Author: "carol", Bodies: []thread.Body{{Time: "t4", Content: "human reply two"}}},
			},
		}
		createTestThread(t, dir, botThread)

		cmd := ThreadShowCmd{ID: botThread.ID, Authors: authorsHumans, Limit: 2}
		var buf bytes.Buffer
		if err := cmd.run(&buf, dir); err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		out := buf.String()
		// "the last 2 human comments", plus the always-kept opening comment.
		if !strings.Contains(out, "opening concern") {
			t.Errorf("expected the opening comment, got:\n%s", out)
		}
		if !strings.Contains(out, "human reply one") || !strings.Contains(out, "human reply two") {
			t.Errorf("expected the last 2 human comments, got:\n%s", out)
		}
		if strings.Contains(out, "ci noise") {
			t.Errorf("bot comment should be filtered out, got:\n%s", out)
		}
		// 3 humans, 2 shown in the tail, and the opening comment is one of the
		// 3 — so nothing is actually missing between them.
		if strings.Contains(out, "omitted") {
			t.Errorf("nothing should be elided here, got:\n%s", out)
		}
	})
}

func TestThreadCreateCmd(t *testing.T) {
	t.Run("creates thread with message", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		cmd := ThreadCreateCmd{
			Message: "Test message",
			Goal:    "review",
		}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		out := buf.String()
		if !strings.HasPrefix(out, "Created thread l:") {
			t.Errorf("output should start with 'Created thread l:', got: %s", out)
		}

		// Verify thread was actually created
		notaDir := filepath.Join(dir, ".nota")
		entries, err := os.ReadDir(notaDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Errorf("expected 1 thread file, got %d", len(entries))
		}
	})

	t.Run("creates thread with group", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		cmd := ThreadCreateCmd{
			Message: "Grouped thread",
			Goal:    "impl",
			Group:   "my-group",
		}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		// Verify filename includes group
		notaDir := filepath.Join(dir, ".nota")
		entries, err := os.ReadDir(notaDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 file, got %d", len(entries))
		}
		if !strings.HasPrefix(entries[0].Name(), "my-group-") {
			t.Errorf("filename should start with 'my-group-', got: %s", entries[0].Name())
		}
	})

	t.Run("rejects empty message", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		cmd := ThreadCreateCmd{
			Message: "",
			Goal:    "review",
		}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err == nil {
			t.Error("expected error for empty message")
		}
	})

	t.Run("rejects invalid goal", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		cmd := ThreadCreateCmd{
			Message: "Test",
			Goal:    "invalid-goal",
		}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err == nil {
			t.Error("expected error for invalid goal")
		}
	})
}

func TestThreadStatusCommands(t *testing.T) {
	t.Run("resolve", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		th := &thread.Thread{
			ID:     "l:0000001234567890",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
			}},
		}
		createTestThread(t, dir, th)

		cmd := ThreadResolveCmd{ID: th.ID}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		if !strings.Contains(buf.String(), "marked as resolved") {
			t.Errorf("output should confirm resolution, got: %s", buf.String())
		}

		// Verify status changed
		info, _ := thread.FindThread(filepath.Join(dir, ".nota"), th.ID)
		if info.Thread.Status != "resolved" {
			t.Errorf("status = %s, want resolved", info.Thread.Status)
		}
	})

	t.Run("wontfix", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		th := &thread.Thread{
			ID:     "l:000000f123456789",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
			}},
		}
		createTestThread(t, dir, th)

		cmd := ThreadWontfixCmd{ID: th.ID}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		if !strings.Contains(buf.String(), "marked as wontfix") {
			t.Errorf("output should confirm wontfix, got: %s", buf.String())
		}

		info, _ := thread.FindThread(filepath.Join(dir, ".nota"), th.ID)
		if info.Thread.Status != "wontfix" {
			t.Errorf("status = %s, want wontfix", info.Thread.Status)
		}
	})

	t.Run("reopen", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		th := &thread.Thread{
			ID:     "l:00000e1234567890",
			Status: "resolved",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
			}},
		}
		createTestThread(t, dir, th)

		cmd := ThreadReopenCmd{ID: th.ID}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		if !strings.Contains(buf.String(), "marked as open") {
			t.Errorf("output should confirm reopen, got: %s", buf.String())
		}

		info, _ := thread.FindThread(filepath.Join(dir, ".nota"), th.ID)
		if info.Thread.Status != "open" {
			t.Errorf("status = %s, want open", info.Thread.Status)
		}
	})

	t.Run("thread not found", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		cmd := ThreadResolveCmd{ID: "l:nonexistent12345"}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err == nil {
			t.Error("expected error for non-existent thread")
		}
	})
}

func TestThreadGoalCmd(t *testing.T) {
	t.Run("updates goal", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		th := &thread.Thread{
			ID:     "l:000a0e1234567890",
			Status: "open",
			Goal:   "review",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
			}},
		}
		createTestThread(t, dir, th)

		cmd := ThreadGoalCmd{ID: th.ID, Goal: "impl"}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		if !strings.Contains(buf.String(), "goal set to impl") {
			t.Errorf("output should confirm goal change, got: %s", buf.String())
		}

		info, _ := thread.FindThread(filepath.Join(dir, ".nota"), th.ID)
		if info.Thread.Goal != "impl" {
			t.Errorf("goal = %s, want impl", info.Thread.Goal)
		}
	})

	t.Run("rejects invalid goal", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		th := &thread.Thread{
			ID:     "l:000bad1234567890",
			Status: "open",
			Goal:   "review",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
			}},
		}
		createTestThread(t, dir, th)

		cmd := ThreadGoalCmd{ID: th.ID, Goal: "not-a-goal"}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err == nil {
			t.Error("expected error for invalid goal")
		}
	})
}

func TestThreadAddCmd(t *testing.T) {
	t.Run("adds comment to thread", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		th := &thread.Thread{
			ID:     "l:00add01234567890",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Initial comment"}},
			}},
		}
		createTestThread(t, dir, th)

		cmd := ThreadAddCmd{ID: th.ID, Message: "Reply comment"}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		if !strings.Contains(buf.String(), "Added comment") {
			t.Errorf("output should confirm comment added, got: %s", buf.String())
		}

		// Verify comment was added
		info, _ := thread.FindThread(filepath.Join(dir, ".nota"), th.ID)
		if len(info.Thread.Comments) != 2 {
			t.Errorf("comments = %d, want 2", len(info.Thread.Comments))
		}
	})

	t.Run("adds comment with local visibility", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		th := &thread.Thread{
			ID:     "l:10ca1234567890ab",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Initial"}},
			}},
		}
		createTestThread(t, dir, th)

		cmd := ThreadAddCmd{ID: th.ID, Message: "Local comment", Local: true}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		info, _ := thread.FindThread(filepath.Join(dir, ".nota"), th.ID)
		if info.Thread.Comments[1].Visibility != "local" {
			t.Errorf("visibility = %s, want local", info.Thread.Comments[1].Visibility)
		}
	})

	t.Run("adds comment with reply-to", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		th := &thread.Thread{
			ID:     "l:0e01234567890abc",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Initial"}},
			}},
		}
		createTestThread(t, dir, th)

		cmd := ThreadAddCmd{ID: th.ID, Message: "Reply to alice", ReplyTo: "l:c001c001c001c001"}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		info, _ := thread.FindThread(filepath.Join(dir, ".nota"), th.ID)
		if info.Thread.Comments[1].ReplyTo == nil || info.Thread.Comments[1].ReplyTo.Ref != "l:c001c001c001c001" {
			t.Errorf("reply-to not set correctly")
		}
	})

	t.Run("thread not found", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		cmd := ThreadAddCmd{ID: "l:nonexistent12345", Message: "Test"}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err == nil {
			t.Error("expected error for non-existent thread")
		}
	})

	t.Run("rejects empty message", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		th := &thread.Thread{
			ID:     "l:e0012345678abcde",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Initial"}},
			}},
		}
		createTestThread(t, dir, th)

		cmd := ThreadAddCmd{ID: th.ID, Message: ""}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err == nil {
			t.Error("expected error for empty message")
		}
	})

	t.Run("rejects message and file together", func(t *testing.T) {
		cmd := ThreadAddCmd{
			ID:      "l:test1234567890ab",
			Message: "hello",
			File:    "somefile.txt",
		}
		_, err := cmd.resolveMessage()
		if err == nil {
			t.Error("expected error when both message and file are set")
		}
	})

	t.Run("reads from file", func(t *testing.T) {
		dir := setupThreadTestDir(t)
		msgFile := filepath.Join(dir, "message.txt")
		if err := os.WriteFile(msgFile, []byte("Message from file"), 0o644); err != nil {
			t.Fatal(err)
		}

		th := &thread.Thread{
			ID:     "l:f01234567890abcd",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Initial"}},
			}},
		}
		createTestThread(t, dir, th)

		cmd := ThreadAddCmd{ID: th.ID, File: msgFile}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		info, _ := thread.FindThread(filepath.Join(dir, ".nota"), th.ID)
		if info.Thread.Comments[1].Bodies[0].Content != "Message from file" {
			t.Errorf("content = %s, want 'Message from file'", info.Thread.Comments[1].Bodies[0].Content)
		}
	})
}

func TestThreadSpawnCmd(t *testing.T) {
	t.Run("creates child thread with parent reference", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		parent := &thread.Thread{
			ID:     "l:0a0e1234567890ab",
			Status: "open",
			Group:  "test-group",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Parent thread"}},
			}},
		}
		createTestThread(t, dir, parent)

		cmd := ThreadSpawnCmd{ParentID: parent.ID, Message: "Child message", Goal: "impl"}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		out := buf.String()
		if !strings.Contains(out, "Created child thread") {
			t.Errorf("output should confirm child creation, got: %s", out)
		}
		if !strings.Contains(out, parent.ID) {
			t.Errorf("output should reference parent ID, got: %s", out)
		}

		// Verify child was created with parent reference
		notaDir := filepath.Join(dir, ".nota")
		entries, _ := os.ReadDir(notaDir)
		if len(entries) != 2 {
			t.Fatalf("expected 2 thread files, got %d", len(entries))
		}
	})

	t.Run("inherits group from parent", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		parent := &thread.Thread{
			ID:     "l:0a0e123456789012",
			Status: "open",
			Group:  "inherited-group",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Parent"}},
			}},
		}
		createTestThread(t, dir, parent)

		cmd := ThreadSpawnCmd{ParentID: parent.ID, Message: "Child"}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		// Find child thread and verify group
		notaDir := filepath.Join(dir, ".nota")
		infos, _ := thread.ListThreads(notaDir, thread.ThreadFilter{})
		found := false
		for _, info := range infos {
			if info.Thread.Parent != nil && info.Thread.Parent.Ref == parent.ID {
				found = true
				if info.Thread.Group != "inherited-group" {
					t.Errorf("child group = %s, want inherited-group", info.Thread.Group)
				}
			}
		}
		if !found {
			t.Fatal("child thread with parent reference not found")
		}
	})

	t.Run("parent not found", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		cmd := ThreadSpawnCmd{ParentID: "l:nonexistent12345", Message: "Child"}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err == nil {
			t.Error("expected error for non-existent parent")
		}
	})
}

func TestThreadDependCmd(t *testing.T) {
	t.Run("adds single dependency", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		th := &thread.Thread{
			ID:     "l:de0e1d1234567890",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
			}},
		}
		createTestThread(t, dir, th)

		blocker := &thread.Thread{
			ID:     "l:b10c1234567890ab",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c002c002c002c002", Author: "bob",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Blocker"}},
			}},
		}
		createTestThread(t, dir, blocker)

		cmd := ThreadDependCmd{ID: th.ID, BlockerIDs: []string{blocker.ID}}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		if !strings.Contains(buf.String(), "now depends on") {
			t.Errorf("output should confirm dependency, got: %s", buf.String())
		}

		info, _ := thread.FindThread(filepath.Join(dir, ".nota"), th.ID)
		if info.Thread.DependsOn != blocker.ID {
			t.Errorf("depends-on = %s, want %s", info.Thread.DependsOn, blocker.ID)
		}
	})

	t.Run("adds multiple dependencies", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		th := &thread.Thread{
			ID:     "l:0de012345678abcd",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
			}},
		}
		createTestThread(t, dir, th)

		blocker1 := &thread.Thread{
			ID:     "l:b10c1234567890a1",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c002c002c002c002", Author: "bob",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Blocker1"}},
			}},
		}
		createTestThread(t, dir, blocker1)

		blocker2 := &thread.Thread{
			ID:     "l:b10c1234567890b2",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c003c003c003c003", Author: "carol",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Blocker2"}},
			}},
		}
		createTestThread(t, dir, blocker2)

		cmd := ThreadDependCmd{ID: th.ID, BlockerIDs: []string{blocker1.ID, blocker2.ID}}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		info, _ := thread.FindThread(filepath.Join(dir, ".nota"), th.ID)
		deps := thread.ParseDependsOn(info.Thread.DependsOn)
		if len(deps) != 2 {
			t.Errorf("expected 2 dependencies, got %d", len(deps))
		}
	})
}

func TestThreadUndependCmd(t *testing.T) {
	t.Run("removes single dependency", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		blocker := &thread.Thread{
			ID:     "l:b10c1234567890cd",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c002c002c002c002", Author: "bob",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Blocker"}},
			}},
		}
		createTestThread(t, dir, blocker)

		th := &thread.Thread{
			ID:        "l:0de0e12345678abc",
			Status:    "open",
			DependsOn: blocker.ID,
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
			}},
		}
		createTestThread(t, dir, th)

		cmd := ThreadUndependCmd{ID: th.ID, BlockerIDs: []string{blocker.ID}}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		if !strings.Contains(buf.String(), "has no dependencies") {
			t.Errorf("output should indicate no dependencies, got: %s", buf.String())
		}

		info, _ := thread.FindThread(filepath.Join(dir, ".nota"), th.ID)
		if info.Thread.DependsOn != "" {
			t.Errorf("depends-on = %s, want empty", info.Thread.DependsOn)
		}
	})

	t.Run("removes one of multiple dependencies", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		blocker1 := &thread.Thread{
			ID:     "l:b10c1234567890e1",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c002c002c002c002", Author: "bob",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Blocker1"}},
			}},
		}
		createTestThread(t, dir, blocker1)

		blocker2 := &thread.Thread{
			ID:     "l:b10c1234567890f2",
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c003c003c003c003", Author: "carol",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Blocker2"}},
			}},
		}
		createTestThread(t, dir, blocker2)

		th := &thread.Thread{
			ID:        "l:0de0e1234567890a",
			Status:    "open",
			DependsOn: blocker1.ID + "," + blocker2.ID,
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
			}},
		}
		createTestThread(t, dir, th)

		cmd := ThreadUndependCmd{ID: th.ID, BlockerIDs: []string{blocker1.ID}}
		var buf bytes.Buffer
		err := cmd.run(&buf, dir)
		if err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		if !strings.Contains(buf.String(), "now depends on") {
			t.Errorf("output should show remaining dependency, got: %s", buf.String())
		}

		info, _ := thread.FindThread(filepath.Join(dir, ".nota"), th.ID)
		if info.Thread.DependsOn != blocker2.ID {
			t.Errorf("depends-on = %s, want %s", info.Thread.DependsOn, blocker2.ID)
		}
	})
}

func TestNewThread(t *testing.T) {
	th := thread.NewThread("open", "review")

	// ID should be exactly 18 chars: "l:" + 16 hex chars
	if len(th.ID) != 18 {
		t.Errorf("ID length = %d, want 18", len(th.ID))
	}

	// ID should match format l:[0-9a-f]{16}
	matched, _ := regexp.MatchString(`^l:[0-9a-f]{16}$`, th.ID)
	if !matched {
		t.Errorf("ID %q does not match format l:[0-9a-f]{16}", th.ID)
	}

	if th.Status != "open" {
		t.Errorf("Status = %s, want open", th.Status)
	}
	if th.Goal != "review" {
		t.Errorf("Goal = %s, want review", th.Goal)
	}
}

func TestAgentAuthorship(t *testing.T) {
	// lastComment returns the most recently appended comment of a thread.
	lastComment := func(t *testing.T, dir, id string) thread.Comment {
		t.Helper()
		info, err := thread.FindThread(filepath.Join(dir, ".nota"), id)
		if err != nil || info == nil {
			t.Fatalf("FindThread(%s) failed: %v", id, err)
		}
		return info.Thread.Comments[len(info.Thread.Comments)-1]
	}

	// newParent creates a thread to comment on or spawn from.
	newParent := func(t *testing.T, dir, id string) *thread.Thread {
		t.Helper()
		th := &thread.Thread{
			ID:     id,
			Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Initial"}},
			}},
		}
		createTestThread(t, dir, th)
		return th
	}

	t.Run("add records agent author when --agent is set", func(t *testing.T) {
		dir := setupThreadTestDir(t)
		th := newParent(t, dir, "l:a9e0000000000001")

		cmd := ThreadAddCmd{ID: th.ID, Message: "Agent reply", Agent: true}
		if err := cmd.run(&bytes.Buffer{}, dir); err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		if got := lastComment(t, dir, th.ID).Author; got != AgentAuthor {
			t.Errorf("Author = %q, want %q", got, AgentAuthor)
		}
	})

	t.Run("add falls back to git user without --agent", func(t *testing.T) {
		dir := setupThreadTestDir(t)
		th := newParent(t, dir, "l:a9e0000000000002")

		cmd := ThreadAddCmd{ID: th.ID, Message: "Human reply"}
		if err := cmd.run(&bytes.Buffer{}, dir); err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		if got := lastComment(t, dir, th.ID).Author; got == AgentAuthor {
			t.Errorf("Author = %q, want the git user, not the agent", got)
		}
	})

	t.Run("create records agent author when --agent is set", func(t *testing.T) {
		dir := setupThreadTestDir(t)

		cmd := ThreadCreateCmd{Message: "Agent-filed finding", Goal: "review", Agent: true}
		if err := cmd.run(&bytes.Buffer{}, dir); err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		threads, err := thread.ListThreads(filepath.Join(dir, ".nota"), thread.ThreadFilter{})
		if err != nil || len(threads) != 1 {
			t.Fatalf("ListThreads() = %d threads, err %v; want 1", len(threads), err)
		}
		if got := threads[0].Thread.Comments[0].Author; got != AgentAuthor {
			t.Errorf("Author = %q, want %q", got, AgentAuthor)
		}
	})

	t.Run("spawn records agent author when --agent is set", func(t *testing.T) {
		dir := setupThreadTestDir(t)
		parent := newParent(t, dir, "l:a9e0000000000003")

		cmd := ThreadSpawnCmd{ParentID: parent.ID, Message: "Agent follow-up", Agent: true}
		var buf bytes.Buffer
		if err := cmd.run(&buf, dir); err != nil {
			t.Fatalf("run() failed: %v", err)
		}

		// The child is the thread that is not the parent.
		threads, _ := thread.ListThreads(filepath.Join(dir, ".nota"), thread.ThreadFilter{})
		var child *thread.Thread
		for _, info := range threads {
			if info.Thread.ID != parent.ID {
				child = info.Thread
			}
		}
		if child == nil {
			t.Fatal("child thread not created")
		}
		if got := child.Comments[0].Author; got != AgentAuthor {
			t.Errorf("Author = %q, want %q", got, AgentAuthor)
		}
	})
}
