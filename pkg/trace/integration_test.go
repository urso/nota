package trace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/urso/nota/pkg/thread"
)

func TestTraceAnchorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temp git repo
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// Create initial file
	filePath := filepath.Join(dir, "test.go")
	writeFile(t, filePath, `package main

func main() {
	fmt.Println("line 4")
	fmt.Println("line 5")
	fmt.Println("line 6")
}
`)
	runGit(t, dir, "add", "test.go")
	runGit(t, dir, "commit", "-m", "initial")
	commit1 := getHead(t, dir)

	// Create anchor at line 5
	anchor := thread.Anchor{
		File:        "test.go",
		Line:        5,
		Commit:      commit1,
		ContentHash: computeContentHash([]byte(`package main

func main() {
	fmt.Println("line 4")
	fmt.Println("line 5")
	fmt.Println("line 6")
}
`), 5),
	}

	// Add lines at the top
	writeFile(t, filePath, `package main

import "fmt"

func main() {
	fmt.Println("line 4")
	fmt.Println("line 5")
	fmt.Println("line 6")
}
`)
	runGit(t, dir, "add", "test.go")
	runGit(t, dir, "commit", "-m", "add import")
	commit2 := getHead(t, dir)

	// Trace anchor
	result, err := TraceAnchor(dir, anchor, commit2)
	if err != nil {
		t.Fatalf("TraceAnchor failed: %v", err)
	}

	// Line should have moved from 5 to 7 (2 lines added)
	if result.Anchor.Line != 7 {
		t.Errorf("expected line 7, got %d", result.Anchor.Line)
	}
	if result.Anchor.TracedFrom != commit1 {
		t.Errorf("expected traced-from %s, got %s", commit1, result.Anchor.TracedFrom)
	}
	if result.Anchor.Commit != commit2 {
		t.Errorf("expected commit %s, got %s", commit2, result.Anchor.Commit)
	}
	if result.Outdated {
		t.Error("expected not outdated")
	}
}

func TestTraceAnchorDeletedLine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	filePath := filepath.Join(dir, "test.go")
	writeFile(t, filePath, `line1
line2
line3
line4
line5
`)
	runGit(t, dir, "add", "test.go")
	runGit(t, dir, "commit", "-m", "initial")
	commit1 := getHead(t, dir)

	anchor := thread.Anchor{
		File:   "test.go",
		Line:   3,
		Commit: commit1,
	}

	// Delete line 3
	writeFile(t, filePath, `line1
line2
line4
line5
`)
	runGit(t, dir, "add", "test.go")
	runGit(t, dir, "commit", "-m", "delete line 3")
	commit2 := getHead(t, dir)

	result, err := TraceAnchor(dir, anchor, commit2)
	if err != nil {
		t.Fatalf("TraceAnchor failed: %v", err)
	}

	if !result.Outdated {
		t.Error("expected outdated for deleted line")
	}
	if !result.Anchor.Outdated {
		t.Error("expected anchor marked outdated")
	}
}

func TestTraceAnchorFileRenamed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	filePath := filepath.Join(dir, "old.go")
	writeFile(t, filePath, `line1
line2
line3
`)
	runGit(t, dir, "add", "old.go")
	runGit(t, dir, "commit", "-m", "initial")
	commit1 := getHead(t, dir)

	anchor := thread.Anchor{
		File:   "old.go",
		Line:   2,
		Commit: commit1,
	}

	// Rename file
	runGit(t, dir, "mv", "old.go", "new.go")
	runGit(t, dir, "commit", "-m", "rename file")
	commit2 := getHead(t, dir)

	result, err := TraceAnchor(dir, anchor, commit2)
	if err != nil {
		t.Fatalf("TraceAnchor failed: %v", err)
	}

	// git log --follow tracks renames, so the anchor should update to new filename.
	// If rename tracking fails, the anchor is marked outdated (acceptable fallback
	// since the old file no longer exists). Either outcome is valid.
	if result.Anchor.File == "new.go" {
		// Rename was followed successfully
		if result.Anchor.Line != 2 {
			t.Errorf("expected line 2, got %d", result.Anchor.Line)
		}
		if result.Outdated {
			t.Error("rename followed but marked outdated unexpectedly")
		}
		t.Log("rename tracking succeeded")
	} else if result.Outdated {
		// Rename not followed, marked outdated (acceptable)
		t.Log("rename not followed, anchor marked outdated (acceptable fallback)")
	} else {
		t.Errorf("unexpected result: file=%q line=%d outdated=%v; want file=new.go or outdated=true",
			result.Anchor.File, result.Anchor.Line, result.Outdated)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2020-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2020-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func getHead(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD failed: %v", err)
	}
	return string(out[:len(out)-1]) // trim newline
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

func TestTraceToWorkingTree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	filePath := filepath.Join(dir, "test.go")
	writeFile(t, filePath, `line1
line2
line3
line4
line5
`)
	runGit(t, dir, "add", "test.go")
	runGit(t, dir, "commit", "-m", "initial")
	commit1 := getHead(t, dir)

	anchor := thread.Anchor{
		File:   "test.go",
		Line:   3,
		Commit: commit1,
	}

	// Make uncommitted changes: add 2 lines at top
	writeFile(t, filePath, `new line 1
new line 2
line1
line2
line3
line4
line5
`)

	result, err := TraceToWorkingTree(dir, anchor)
	if err != nil {
		t.Fatalf("TraceToWorkingTree failed: %v", err)
	}

	// Line 3 should now be at line 5 (2 lines added)
	if result.Anchor.Line != 5 {
		t.Errorf("expected line 5, got %d", result.Anchor.Line)
	}
	if result.Anchor.Commit != "" {
		t.Errorf("expected empty commit for working tree, got %q", result.Anchor.Commit)
	}
	if result.Anchor.TracedFrom != commit1 {
		t.Errorf("expected traced-from %s, got %s", commit1, result.Anchor.TracedFrom)
	}
	if result.Outdated {
		t.Error("expected not outdated")
	}
}

func TestTraceToWorkingTreeDeletedLine(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	filePath := filepath.Join(dir, "test.go")
	writeFile(t, filePath, `line1
line2
line3
line4
`)
	runGit(t, dir, "add", "test.go")
	runGit(t, dir, "commit", "-m", "initial")
	commit1 := getHead(t, dir)

	anchor := thread.Anchor{
		File:   "test.go",
		Line:   2,
		Commit: commit1,
	}

	// Delete line 2 in working tree
	writeFile(t, filePath, `line1
line3
line4
`)

	result, err := TraceToWorkingTree(dir, anchor)
	if err != nil {
		t.Fatalf("TraceToWorkingTree failed: %v", err)
	}

	if !result.Outdated {
		t.Error("expected outdated for deleted line")
	}
}

func TestTraceWithBacktrack(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	filePath := filepath.Join(dir, "test.go")
	writeFile(t, filePath, `line1
line2
line3
`)
	runGit(t, dir, "add", "test.go")
	runGit(t, dir, "commit", "-m", "commit1")
	commit1 := getHead(t, dir)

	// Anchor at commit1, line 2
	anchor1 := thread.Anchor{
		File:   "test.go",
		Line:   2,
		Commit: commit1,
	}

	// Add lines and create commit2
	writeFile(t, filePath, `line1
new line
line2
line3
`)
	runGit(t, dir, "add", "test.go")
	runGit(t, dir, "commit", "-m", "commit2")
	commit2 := getHead(t, dir)

	// Anchor traced to commit2, line 3
	anchor2 := thread.Anchor{
		File:       "test.go",
		Line:       3,
		Commit:     commit2,
		TracedFrom: commit1,
	}

	// Revert to commit1
	runGit(t, dir, "reset", "--hard", commit1)

	// Add different content
	writeFile(t, filePath, `line1
line2
line3
extra line
`)
	runGit(t, dir, "add", "test.go")
	runGit(t, dir, "commit", "-m", "commit3 after revert")
	commit3 := getHead(t, dir)

	// anchor2 is at commit2, which is NOT an ancestor of commit3 (we reverted)
	// Backtracking should find anchor1 (at commit1, which IS an ancestor of commit3)
	// and trace from there
	anchors := []thread.Anchor{anchor1, anchor2}

	result, err := TraceWithBacktrack(dir, anchors, commit3)
	if err != nil {
		t.Fatalf("TraceWithBacktrack failed: %v", err)
	}

	// Line 2 should still be at line 2 (no change to that line)
	if result.Anchor.Line != 2 {
		t.Errorf("expected line 2, got %d", result.Anchor.Line)
	}
	if result.Anchor.Commit != commit3 {
		t.Errorf("expected commit %s, got %s", commit3, result.Anchor.Commit)
	}
	if result.Outdated {
		t.Error("expected not outdated")
	}
}

func TestTraceAnchors(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// Create two files
	file1 := filepath.Join(dir, "file1.go")
	file2 := filepath.Join(dir, "file2.go")
	writeFile(t, file1, `line1
line2
line3
`)
	writeFile(t, file2, `a
b
c
`)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	commit1 := getHead(t, dir)

	// Multiple anchors in same file and different files
	anchors := []thread.Anchor{
		{File: "file1.go", Line: 1, Commit: commit1},
		{File: "file1.go", Line: 3, Commit: commit1},
		{File: "file2.go", Line: 2, Commit: commit1},
	}

	// Add lines to both files
	writeFile(t, file1, `new header
line1
line2
line3
`)
	writeFile(t, file2, `a
new b
b
c
`)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "add lines")
	commit2 := getHead(t, dir)

	results := TraceAnchors(dir, anchors, commit2)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// file1.go: line 1 -> line 2, line 3 -> line 4
	if results[0].Error != nil {
		t.Errorf("result[0] error: %v", results[0].Error)
	} else if results[0].Result.Anchor.Line != 2 {
		t.Errorf("result[0] expected line 2, got %d", results[0].Result.Anchor.Line)
	}

	if results[1].Error != nil {
		t.Errorf("result[1] error: %v", results[1].Error)
	} else if results[1].Result.Anchor.Line != 4 {
		t.Errorf("result[1] expected line 4, got %d", results[1].Result.Anchor.Line)
	}

	// file2.go: line 2 -> line 3
	if results[2].Error != nil {
		t.Errorf("result[2] error: %v", results[2].Error)
	} else if results[2].Result.Anchor.Line != 3 {
		t.Errorf("result[2] expected line 3, got %d", results[2].Result.Anchor.Line)
	}
}
