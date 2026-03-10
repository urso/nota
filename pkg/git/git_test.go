package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func setupGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "test")

	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func TestListFiles(t *testing.T) {
	dir := setupGitRepo(t)

	// Create and commit a file.
	writeFile(t, dir, "committed.go", "package main\n")
	gitCmd(t, dir, "add", "committed.go")
	gitCmd(t, dir, "commit", "-m", "initial")

	// Create a staged file.
	writeFile(t, dir, "staged.go", "package staged\n")
	gitCmd(t, dir, "add", "staged.go")

	// Create an unstaged modification.
	writeFile(t, dir, "committed.go", "package main\n// modified\n")

	// Create an untracked file.
	writeFile(t, dir, "untracked.go", "package untracked\n")

	t.Run("ScopeModified", func(t *testing.T) {
		files, err := ListFiles(ScopeModified, dir)
		if err != nil {
			t.Fatal(err)
		}
		expected := []string{"committed.go", "staged.go", "untracked.go"}
		if diff := cmp.Diff(expected, files); diff != "" {
			t.Errorf("files mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("ScopeStaged", func(t *testing.T) {
		files, err := ListFiles(ScopeStaged, dir)
		if err != nil {
			t.Fatal(err)
		}
		expected := []string{"staged.go"}
		if diff := cmp.Diff(expected, files); diff != "" {
			t.Errorf("files mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("ScopeUnstaged", func(t *testing.T) {
		files, err := ListFiles(ScopeUnstaged, dir)
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(files)
		expected := []string{"committed.go", "untracked.go"}
		if diff := cmp.Diff(expected, files); diff != "" {
			t.Errorf("files mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("ScopeAll", func(t *testing.T) {
		files, err := ListFiles(ScopeAll, dir)
		if err != nil {
			t.Fatal(err)
		}
		expected := []string{"committed.go", "staged.go", "untracked.go"}
		if diff := cmp.Diff(expected, files); diff != "" {
			t.Errorf("files mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("non-git directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := ListFiles(ScopeAll, tmpDir)
		if err == nil {
			t.Error("expected error for non-git directory")
		}
	})

	t.Run("deleted file excluded", func(t *testing.T) {
		// Remove a tracked file without staging the deletion.
		if err := os.Remove(filepath.Join(dir, "committed.go")); err != nil {
			t.Fatal(err)
		}
		files, err := ListFiles(ScopeModified, dir)
		if err != nil {
			t.Fatal(err)
		}
		// committed.go should NOT be in results since the file doesn't exist.
		for _, f := range files {
			if f == "committed.go" {
				t.Error("deleted file committed.go should not be in results")
			}
		}
		// Restore it for subsequent tests.
		writeFile(t, dir, "committed.go", "package main\n// modified\n")
	})

	t.Run("deduplication", func(t *testing.T) {
		// A file that's both staged and has unstaged changes appears once.
		writeFile(t, dir, "staged.go", "package staged\n// also unstaged\n")
		files, err := ListFiles(ScopeModified, dir)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, f := range files {
			if f == "staged.go" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("staged.go appeared %d times, want 1", count)
		}
	})
}

func TestIsAvailable(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git not available")
	}
}
