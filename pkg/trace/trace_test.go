package trace

import (
	"testing"

	"github.com/urso/nota/pkg/thread"
)

func TestParseHunkHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected Hunk
	}{
		{"@@ -10,5 +12,7 @@", Hunk{OldStart: 10, OldCount: 5, NewStart: 12, NewCount: 7}},
		{"@@ -1 +1 @@", Hunk{OldStart: 1, OldCount: 1, NewStart: 1, NewCount: 1}},
		{"@@ -0,0 +1,3 @@", Hunk{OldStart: 0, OldCount: 0, NewStart: 1, NewCount: 3}},
		{"@@ -5,3 +5,0 @@", Hunk{OldStart: 5, OldCount: 3, NewStart: 5, NewCount: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseHunkHeader(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.OldStart != tt.expected.OldStart || got.OldCount != tt.expected.OldCount ||
				got.NewStart != tt.expected.NewStart || got.NewCount != tt.expected.NewCount {
				t.Errorf("got %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestApplyHunks(t *testing.T) {
	tests := []struct {
		name     string
		line     int
		hunks    []Hunk
		expected int
	}{
		{
			name:     "no hunks",
			line:     10,
			hunks:    nil,
			expected: 10,
		},
		{
			name:     "line before hunk",
			line:     5,
			hunks:    []Hunk{{OldStart: 10, OldCount: 3, NewStart: 10, NewCount: 5}},
			expected: 5,
		},
		{
			name:     "line after hunk with additions",
			line:     20,
			hunks:    []Hunk{{OldStart: 10, OldCount: 3, NewStart: 10, NewCount: 5}},
			expected: 22, // 20 + (5-3)
		},
		{
			name:     "line after hunk with deletions",
			line:     20,
			hunks:    []Hunk{{OldStart: 10, OldCount: 5, NewStart: 10, NewCount: 2}},
			expected: 17, // 20 + (2-5)
		},
		{
			name: "line within deleted range with diff lines",
			line: 12,
			hunks: []Hunk{{
				OldStart: 10, OldCount: 5, NewStart: 10, NewCount: 0,
				Lines: []DiffLine{
					{Type: '-', Content: "line 10"},
					{Type: '-', Content: "line 11"},
					{Type: '-', Content: "line 12"},
					{Type: '-', Content: "line 13"},
					{Type: '-', Content: "line 14"},
				},
			}},
			expected: 0, // deleted
		},
		{
			name: "line survives in modified hunk",
			line: 11,
			hunks: []Hunk{{
				OldStart: 10, OldCount: 3, NewStart: 10, NewCount: 5,
				Lines: []DiffLine{
					{Type: ' ', Content: "context"},  // old 10 -> new 10
					{Type: '+', Content: "added 1"},  // new 11
					{Type: '+', Content: "added 2"},  // new 12
					{Type: ' ', Content: "original"}, // old 11 -> new 13
					{Type: ' ', Content: "another"},  // old 12 -> new 14
				},
			}},
			expected: 13, // line 11 maps to new line 13
		},
		{
			name: "multiple hunks",
			line: 30,
			hunks: []Hunk{
				{OldStart: 5, OldCount: 2, NewStart: 5, NewCount: 4},   // +2
				{OldStart: 20, OldCount: 3, NewStart: 22, NewCount: 1}, // -2
			},
			expected: 30, // +2 -2 = 0 net change
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyHunks(tt.line, tt.hunks)
			if got != tt.expected {
				t.Errorf("applyHunks(%d, %v) = %d, want %d", tt.line, tt.hunks, got, tt.expected)
			}
		})
	}
}

func TestParseDiffs(t *testing.T) {
	input := []byte(`abc123
diff --git a/file.go b/file.go
--- a/file.go
+++ b/file.go
@@ -10,3 +10,5 @@
 context
-removed
+added
+more
def456
diff --git a/old.go b/new.go
--- a/old.go
+++ b/new.go
@@ -1,2 +1,3 @@
 line
`)

	diffs, err := parseDiffs(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diffs) != 2 {
		t.Fatalf("expected 2 diffs, got %d", len(diffs))
	}

	// First diff
	if diffs[0].OldName != "file.go" {
		t.Errorf("diff 0: expected old name 'file.go', got %q", diffs[0].OldName)
	}
	if diffs[0].NewName != "file.go" {
		t.Errorf("diff 0: expected new name 'file.go', got %q", diffs[0].NewName)
	}
	if len(diffs[0].Hunks) != 1 {
		t.Fatalf("diff 0: expected 1 hunk, got %d", len(diffs[0].Hunks))
	}
	if diffs[0].Hunks[0].OldStart != 10 {
		t.Errorf("diff 0 hunk 0: expected old start 10, got %d", diffs[0].Hunks[0].OldStart)
	}

	// Second diff (rename)
	if diffs[1].OldName != "old.go" {
		t.Errorf("diff 1: expected old name 'old.go', got %q", diffs[1].OldName)
	}
	if diffs[1].NewName != "new.go" {
		t.Errorf("diff 1: expected new name 'new.go', got %q", diffs[1].NewName)
	}
}

func TestParseDiffsDeletedFile(t *testing.T) {
	input := []byte(`abc123
diff --git a/file.go b/file.go
--- a/file.go
+++ /dev/null
@@ -1,10 +0,0 @@
-deleted content
`)

	diffs, err := parseDiffs(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}

	if !diffs[0].Deleted {
		t.Error("expected diff to be marked as deleted")
	}
}

func TestComputeContentHash(t *testing.T) {
	content := []byte("line1\nline2\nline3\n")

	hash1 := computeContentHash(content, 1)
	hash2 := computeContentHash(content, 2)
	hash3 := computeContentHash(content, 3)

	if hash1 == "" || hash2 == "" || hash3 == "" {
		t.Error("expected non-empty hashes")
	}

	if hash1 == hash2 || hash2 == hash3 {
		t.Error("expected different hashes for different lines")
	}

	// Same content should produce same hash
	hash1Again := computeContentHash(content, 1)
	if hash1 != hash1Again {
		t.Error("expected same hash for same content")
	}

	// Hash should be 8 hex chars
	if len(hash1) != 8 {
		t.Errorf("expected 8 char hash, got %d chars", len(hash1))
	}
}

func TestComputeContentHashOutOfBounds(t *testing.T) {
	content := []byte("line1\nline2\n")

	if h := computeContentHash(content, 0); h != "" {
		t.Error("expected empty hash for line 0")
	}
	if h := computeContentHash(content, 10); h != "" {
		t.Error("expected empty hash for out-of-bounds line")
	}
}

func TestTraceAnchorSameCommit(t *testing.T) {
	anchor := thread.Anchor{
		File:   "test.go",
		Line:   10,
		Commit: "abc123",
	}

	// repoDir is unused in same-commit fast path, but use TempDir for safety
	result, err := TraceAnchor(t.TempDir(), anchor, "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outdated {
		t.Error("expected not outdated for same commit")
	}
	if result.Anchor.Line != 10 {
		t.Errorf("expected line 10, got %d", result.Anchor.Line)
	}
}

func TestTraceAnchorEmptyCommit(t *testing.T) {
	anchor := thread.Anchor{
		File: "test.go",
		Line: 10,
	}

	// repoDir is unused in empty-commit fast path, but use TempDir for safety
	result, err := TraceAnchor(t.TempDir(), anchor, "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outdated {
		t.Error("expected not outdated for empty commit")
	}
	if result.Anchor.Line != 10 {
		t.Errorf("expected line 10, got %d", result.Anchor.Line)
	}
}

func TestCachedGitOps(t *testing.T) {
	t.Run("IsAncestorOf", func(t *testing.T) {
		calls := 0
		mock := &mockGitOps{
			isAncestorFn: func(a, d string) (bool, error) {
				calls++
				return a == "commit1", nil
			},
		}

		cached := NewCachedGitOps(mock)

		// First call should hit the mock
		result1, _ := cached.IsAncestorOf("commit1", "commit2")
		if !result1 {
			t.Error("expected true for commit1 -> commit2")
		}
		if calls != 1 {
			t.Errorf("expected 1 call, got %d", calls)
		}

		// Second call with same args should use cache
		result2, _ := cached.IsAncestorOf("commit1", "commit2")
		if !result2 {
			t.Error("expected true for cached result")
		}
		if calls != 1 {
			t.Errorf("expected still 1 call (cached), got %d", calls)
		}

		// Different args should call mock
		result3, _ := cached.IsAncestorOf("commit3", "commit2")
		if result3 {
			t.Error("expected false for commit3 -> commit2")
		}
		if calls != 2 {
			t.Errorf("expected 2 calls, got %d", calls)
		}
	})

	t.Run("GetDiffs", func(t *testing.T) {
		calls := 0
		mock := &mockGitOps{
			getDiffsFn: func(from, to, file string) ([]Diff, error) {
				calls++
				return []Diff{{OldName: file, NewName: file}}, nil
			},
		}

		cached := NewCachedGitOps(mock)

		// First call should hit the mock
		diffs1, _ := cached.GetDiffs("c1", "c2", "file.go")
		if len(diffs1) != 1 || diffs1[0].NewName != "file.go" {
			t.Errorf("unexpected result: %v", diffs1)
		}
		if calls != 1 {
			t.Errorf("expected 1 call, got %d", calls)
		}

		// Second call with same args should use cache
		diffs2, _ := cached.GetDiffs("c1", "c2", "file.go")
		if len(diffs2) != 1 {
			t.Error("expected cached result")
		}
		if calls != 1 {
			t.Errorf("expected still 1 call (cached), got %d", calls)
		}

		// Different args should call mock
		_, _ = cached.GetDiffs("c1", "c3", "file.go")
		if calls != 2 {
			t.Errorf("expected 2 calls, got %d", calls)
		}
	})

	t.Run("MergeBase", func(t *testing.T) {
		calls := 0
		mock := &mockGitOps{
			mergeBaseFn: func(c1, c2 string) (string, error) {
				calls++
				return "base-" + c1 + "-" + c2, nil
			},
		}

		cached := NewCachedGitOps(mock)

		// First call should hit the mock
		base1, _ := cached.MergeBase("c1", "c2")
		if base1 != "base-c1-c2" {
			t.Errorf("unexpected result: %s", base1)
		}
		if calls != 1 {
			t.Errorf("expected 1 call, got %d", calls)
		}

		// Second call with same args should use cache
		base2, _ := cached.MergeBase("c1", "c2")
		if base2 != "base-c1-c2" {
			t.Error("expected cached result")
		}
		if calls != 1 {
			t.Errorf("expected still 1 call (cached), got %d", calls)
		}

		// Different args should call mock
		_, _ = cached.MergeBase("c1", "c3")
		if calls != 2 {
			t.Errorf("expected 2 calls, got %d", calls)
		}
	})
}

func TestTraceAnchorsWithGitDivergentBranches(t *testing.T) {
	mergeBaseCalled := false
	mock := &mockGitOps{
		repoDir: "/repo",
		isAncestorFn: func(ancestor, descendant string) (bool, error) {
			// Simulate divergent branches: neither commit is ancestor of the other
			return false, nil
		},
		mergeBaseFn: func(c1, c2 string) (string, error) {
			mergeBaseCalled = true
			return "common-ancestor", nil
		},
		getDiffsFn: func(from, to, file string) ([]Diff, error) {
			if from != "common-ancestor" {
				t.Errorf("expected diff from common-ancestor, got %s", from)
			}
			return []Diff{}, nil
		},
	}

	anchors := []thread.Anchor{
		{File: "file.go", Line: 10, Commit: "branch-a-commit"},
	}

	results := TraceAnchorsWithGit(mock, anchors, "branch-b-head")

	if !mergeBaseCalled {
		t.Error("expected MergeBase to be called for divergent branches")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Errorf("unexpected error: %v", results[0].Error)
	}
}

type mockGitOps struct {
	repoDir      string
	isAncestorFn func(ancestor, descendant string) (bool, error)
	getDiffsFn   func(from, to, file string) ([]Diff, error)
	mergeBaseFn  func(c1, c2 string) (string, error)
}

func (m *mockGitOps) RepoDir() string { return m.repoDir }
func (m *mockGitOps) IsAncestorOf(a, d string) (bool, error) {
	if m.isAncestorFn != nil {
		return m.isAncestorFn(a, d)
	}
	return false, nil
}
func (m *mockGitOps) GetDiffs(from, to, file string) ([]Diff, error) {
	if m.getDiffsFn != nil {
		return m.getDiffsFn(from, to, file)
	}
	return nil, nil
}
func (m *mockGitOps) MergeBase(c1, c2 string) (string, error) {
	if m.mergeBaseFn != nil {
		return m.mergeBaseFn(c1, c2)
	}
	return "", nil
}
