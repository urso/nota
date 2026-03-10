package grouper

import (
	"bytes"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/urso/aireview/pkg/parser"
)

func TestExtractContext(t *testing.T) {
	content := []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\n")

	tests := []struct {
		name      string
		content   []byte
		startLine int
		endLine   int
		maxLines  int
		expected  ContextLines
	}{
		{
			name:      "3 lines each side",
			content:   content,
			startLine: 4,
			endLine:   4,
			maxLines:  3,
			expected: ContextLines{
				Before: []string{"line1", "line2", "line3"},
				After:  []string{"line5", "line6", "line7"},
			},
		},
		{
			name:      "blank line stops before",
			content:   []byte("line1\n\nline3\nline4\nline5\n"),
			startLine: 3,
			endLine:   3,
			maxLines:  3,
			expected: ContextLines{
				Before: nil,
				After:  []string{"line4", "line5"},
			},
		},
		{
			name:      "comment at file start",
			content:   []byte("comment\nline2\nline3\n"),
			startLine: 1,
			endLine:   1,
			maxLines:  3,
			expected: ContextLines{
				Before: nil,
				After:  []string{"line2", "line3"},
			},
		},
		{
			name:      "comment at file end",
			content:   []byte("line1\nline2\ncomment"),
			startLine: 3,
			endLine:   3,
			maxLines:  3,
			expected: ContextLines{
				Before: []string{"line1", "line2"},
				After:  nil,
			},
		},
		{
			name:      "adjacent blank line",
			content:   []byte("\ncomment\n\n"),
			startLine: 2,
			endLine:   2,
			maxLines:  3,
			expected: ContextLines{
				Before: nil,
				After:  nil,
			},
		},
		{
			name:      "maxLines 0",
			content:   content,
			startLine: 4,
			endLine:   4,
			maxLines:  0,
			expected:  ContextLines{},
		},
		{
			name:      "multiline comment context from edges",
			content:   []byte("before\nstart\nmiddle\nend\nafter\n"),
			startLine: 2,
			endLine:   4,
			maxLines:  3,
			expected: ContextLines{
				Before: []string{"before"},
				After:  []string{"after"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractContext(tt.content, tt.startLine, tt.endLine, tt.maxLines)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Errorf("context mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGroupComments(t *testing.T) {
	fileA := []byte("line1\nline2\n// review(auth): check token\nline4\nline5\n")
	fileB := []byte("lineA\n// review(auth): validate\nlineC\n")

	t.Run("two review(auth) from different files", func(t *testing.T) {
		comments := []parser.ReviewComment{
			{Tag: parser.TagReview, Name: "auth", Message: "check token", File: "a.go", Line: 3, EndLine: 3},
			{Tag: parser.TagReview, Name: "auth", Message: "validate", File: "b.go", Line: 2, EndLine: 2},
		}
		files := map[string][]byte{"a.go": fileA, "b.go": fileB}
		groups := GroupComments(comments, files, 3)

		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if groups[0].Name != "auth" {
			t.Errorf("expected name 'auth', got %q", groups[0].Name)
		}
		if len(groups[0].Entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(groups[0].Entries))
		}
	})

	t.Run("review(auth) + see: auth", func(t *testing.T) {
		comments := []parser.ReviewComment{
			{Tag: parser.TagReview, Name: "auth", Message: "check", File: "a.go", Line: 3, EndLine: 3},
			{Tag: parser.TagSee, Name: "auth", File: "b.go", Line: 2, EndLine: 2},
		}
		files := map[string][]byte{"a.go": fileA, "b.go": fileB}
		groups := GroupComments(comments, files, 3)

		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if len(groups[0].Entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(groups[0].Entries))
		}
		if len(groups[0].References) != 1 {
			t.Errorf("expected 1 reference, got %d", len(groups[0].References))
		}
	})

	t.Run("unnamed review", func(t *testing.T) {
		comments := []parser.ReviewComment{
			{Tag: parser.TagReview, Name: "", Message: "standalone", File: "a.go", Line: 3, EndLine: 3},
		}
		files := map[string][]byte{"a.go": fileA}
		groups := GroupComments(comments, files, 3)

		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if groups[0].Name != "" {
			t.Errorf("expected empty name, got %q", groups[0].Name)
		}
	})

	t.Run("discuss(api) + also: api", func(t *testing.T) {
		comments := []parser.ReviewComment{
			{Tag: parser.TagDiscuss, Name: "api", Message: "redesign?", File: "a.go", Line: 3, EndLine: 3},
			{Tag: parser.TagAlso, Name: "api", File: "b.go", Line: 2, EndLine: 2},
		}
		files := map[string][]byte{"a.go": fileA, "b.go": fileB}
		groups := GroupComments(comments, files, 3)

		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if len(groups[0].Entries) != 1 || len(groups[0].References) != 1 {
			t.Errorf("expected 1 entry + 1 ref, got %d entries + %d refs",
				len(groups[0].Entries), len(groups[0].References))
		}
	})

	t.Run("mixed named and unnamed", func(t *testing.T) {
		comments := []parser.ReviewComment{
			{Tag: parser.TagReview, Name: "beta", Message: "named beta", File: "a.go", Line: 3, EndLine: 3},
			{Tag: parser.TagReview, Name: "", Message: "unnamed", File: "b.go", Line: 2, EndLine: 2},
			{Tag: parser.TagReview, Name: "alpha", Message: "named alpha", File: "a.go", Line: 4, EndLine: 4},
		}
		files := map[string][]byte{"a.go": fileA, "b.go": fileB}
		groups := GroupComments(comments, files, 0)

		if len(groups) != 3 {
			t.Fatalf("expected 3 groups, got %d", len(groups))
		}
		// Named first, alphabetical.
		if groups[0].Name != "alpha" {
			t.Errorf("expected first group 'alpha', got %q", groups[0].Name)
		}
		if groups[1].Name != "beta" {
			t.Errorf("expected second group 'beta', got %q", groups[1].Name)
		}
		// Unnamed last.
		if groups[2].Name != "" {
			t.Errorf("expected third group unnamed, got %q", groups[2].Name)
		}
	})

	t.Run("see nonexistent", func(t *testing.T) {
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		comments := []parser.ReviewComment{
			{Tag: parser.TagSee, Name: "nonexistent", File: "a.go", Line: 3, EndLine: 3},
		}
		files := map[string][]byte{"a.go": fileA}
		groups := GroupComments(comments, files, 3)

		_ = w.Close()
		os.Stderr = oldStderr

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)

		if len(groups) != 0 {
			t.Errorf("expected 0 groups, got %d", len(groups))
		}
		if !bytes.Contains(buf.Bytes(), []byte("nonexistent")) {
			t.Error("expected warning about nonexistent group")
		}
	})

	t.Run("critique tag routes to Entries", func(t *testing.T) {
		comments := []parser.ReviewComment{
			{Tag: parser.Tag("critique"), Name: "perf", Message: "check allocation", File: "a.go", Line: 3, EndLine: 3},
		}
		files := map[string][]byte{"a.go": fileA}
		groups := GroupComments(comments, files, 0)

		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if len(groups[0].Entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(groups[0].Entries))
		}
		if groups[0].Entries[0].Tag != parser.Tag("critique") {
			t.Errorf("expected tag critique, got %s", groups[0].Entries[0].Tag)
		}
	})

	t.Run("impl tag routes to Entries", func(t *testing.T) {
		comments := []parser.ReviewComment{
			{Tag: parser.Tag("impl"), Name: "", Message: "implement this", File: "a.go", Line: 3, EndLine: 3},
		}
		files := map[string][]byte{"a.go": fileA}
		groups := GroupComments(comments, files, 0)

		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if len(groups[0].Entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(groups[0].Entries))
		}
	})

	t.Run("multiple tags same name", func(t *testing.T) {
		comments := []parser.ReviewComment{
			{Tag: parser.TagReview, Name: "auth", Message: "review it", File: "a.go", Line: 3, EndLine: 3},
			{Tag: parser.TagDiscuss, Name: "auth", Message: "discuss it", File: "b.go", Line: 2, EndLine: 2},
		}
		files := map[string][]byte{"a.go": fileA, "b.go": fileB}
		groups := GroupComments(comments, files, 0)

		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if len(groups[0].Entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(groups[0].Entries))
		}
	})
}
