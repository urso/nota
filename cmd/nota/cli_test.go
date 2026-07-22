package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urso/nota/pkg/thread"
)

func TestValidateCmd(t *testing.T) {
	t.Run("validates all threads in directory", func(t *testing.T) {
		dir := t.TempDir()
		notaDir := filepath.Join(dir, ".nota")
		if err := os.MkdirAll(notaDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create valid threads
		for i, th := range []*thread.Thread{
			{
				ID: "l:0001000100010001", Status: "open",
				Comments: []thread.Comment{{
					ID: "l:c001c001c001c001", Author: "alice",
					Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test 1"}},
				}},
			},
			{
				ID: "l:0002000200020002", Status: "resolved",
				Comments: []thread.Comment{{
					ID: "l:c002c002c002c002", Author: "bob",
					Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test 2"}},
				}},
			},
		} {
			path := filepath.Join(notaDir, thread.Filename(th))
			if err := thread.WriteThread(path, th); err != nil {
				t.Fatalf("failed to write thread %d: %v", i, err)
			}
		}

		cmd := ValidateCmd{}
		var stdout, stderr bytes.Buffer
		err := cmd.run(&stdout, &stderr, dir)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout.String(), "Validated 2 thread file(s)") {
			t.Errorf("stdout = %q, want 'Validated 2 thread file(s)'", stdout.String())
		}
	})

	t.Run("validates specific files", func(t *testing.T) {
		dir := t.TempDir()
		notaDir := filepath.Join(dir, ".nota")
		if err := os.MkdirAll(notaDir, 0o755); err != nil {
			t.Fatal(err)
		}

		th := &thread.Thread{
			ID: "l:0001000100010001", Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
			}},
		}
		path := filepath.Join(notaDir, thread.Filename(th))
		if err := thread.WriteThread(path, th); err != nil {
			t.Fatal(err)
		}

		cmd := ValidateCmd{Files: []string{path}}
		var stdout, stderr bytes.Buffer
		err := cmd.run(&stdout, &stderr, dir)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout.String(), "Validated 1 thread file(s)") {
			t.Errorf("stdout = %q, want 'Validated 1 thread file(s)'", stdout.String())
		}
	})

	t.Run("reports invalid thread", func(t *testing.T) {
		dir := t.TempDir()
		notaDir := filepath.Join(dir, ".nota")
		if err := os.MkdirAll(notaDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create invalid thread (missing status)
		th := &thread.Thread{
			ID: "l:0001000100010001",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
			}},
		}
		path := filepath.Join(notaDir, "invalid.xml")
		if err := thread.WriteThread(path, th); err != nil {
			t.Fatal(err)
		}

		cmd := ValidateCmd{}
		var stdout, stderr bytes.Buffer
		err := cmd.run(&stdout, &stderr, dir)

		if err == nil {
			t.Error("expected error for invalid thread")
		}
		if !strings.Contains(err.Error(), "validation failed") {
			t.Errorf("error = %v, want 'validation failed'", err)
		}
		if !strings.Contains(stderr.String(), "missing required field: status") {
			t.Errorf("stderr = %q, want error about missing status", stderr.String())
		}
	})

	t.Run("reports malformed XML", func(t *testing.T) {
		dir := t.TempDir()
		notaDir := filepath.Join(dir, ".nota")
		if err := os.MkdirAll(notaDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Write malformed XML
		path := filepath.Join(notaDir, "malformed.xml")
		if err := os.WriteFile(path, []byte("<nota-thread><broken"), 0o644); err != nil {
			t.Fatal(err)
		}

		cmd := ValidateCmd{}
		var stdout, stderr bytes.Buffer
		err := cmd.run(&stdout, &stderr, dir)

		if err == nil {
			t.Error("expected error for malformed XML")
		}
		if !strings.Contains(stderr.String(), "malformed.xml") {
			t.Errorf("stderr = %q, want reference to malformed.xml", stderr.String())
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		notaDir := filepath.Join(dir, ".nota")
		if err := os.MkdirAll(notaDir, 0o755); err != nil {
			t.Fatal(err)
		}

		cmd := ValidateCmd{}
		var stdout, stderr bytes.Buffer
		err := cmd.run(&stdout, &stderr, dir)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(stdout.String(), "No thread files found") {
			t.Errorf("stdout = %q, want 'No thread files found'", stdout.String())
		}
	})

	t.Run("skips non-xml files", func(t *testing.T) {
		dir := t.TempDir()
		notaDir := filepath.Join(dir, ".nota")
		if err := os.MkdirAll(notaDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Write a markdown file (old format)
		mdPath := filepath.Join(notaDir, "old.md")
		if err := os.WriteFile(mdPath, []byte("# Old format"), 0o644); err != nil {
			t.Fatal(err)
		}

		// Write a valid XML thread
		th := &thread.Thread{
			ID: "l:0001000100010001", Status: "open",
			Comments: []thread.Comment{{
				ID: "l:c001c001c001c001", Author: "alice",
				Bodies: []thread.Body{{Time: "2026-07-21T00:00:00Z", Content: "Test"}},
			}},
		}
		xmlPath := filepath.Join(notaDir, thread.Filename(th))
		if err := thread.WriteThread(xmlPath, th); err != nil {
			t.Fatal(err)
		}

		cmd := ValidateCmd{}
		var stdout, stderr bytes.Buffer
		err := cmd.run(&stdout, &stderr, dir)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Should only validate 1 file (the XML), not the markdown
		if !strings.Contains(stdout.String(), "Validated 1 thread file(s)") {
			t.Errorf("stdout = %q, want 'Validated 1 thread file(s)'", stdout.String())
		}
	})
}
