package parser

import (
	"bytes"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/urso/aireview/pkg/scanner"
)

// mockCommentScanner is a mock CommentScanner for testing.
type mockCommentScanner struct {
	comments []*scanner.Comment
	index    int
	config   *scanner.Config
}

func (m *mockCommentScanner) Config() *scanner.Config {
	return m.config
}

func (m *mockCommentScanner) Scan() bool {
	m.index++
	return m.index <= len(m.comments)
}

func (m *mockCommentScanner) Next() *scanner.Comment {
	return m.comments[m.index-1]
}

func (m *mockCommentScanner) Err() error {
	return nil
}

func newMock(comments ...*scanner.Comment) *mockCommentScanner {
	return &mockCommentScanner{
		comments: comments,
		config: &scanner.Config{
			LineComments: []scanner.LineCommentConfig{
				{Start: []rune("//")},
			},
			MultilineComments: []scanner.MultilineCommentConfig{
				{Start: []rune("/*"), End: []rune("*/")},
			},
		},
	}
}

func lineComment(text string, line int, startByte, endByte int64) *scanner.Comment {
	return &scanner.Comment{
		Text:      text,
		Line:      line,
		Multiline: false,
		StartByte: startByte,
		EndByte:   endByte,
		LineConfig: &scanner.LineCommentConfig{
			Start: []rune("//"),
		},
	}
}

func blockComment(text string, line int, startByte, endByte int64) *scanner.Comment {
	return &scanner.Comment{
		Text:      text,
		Line:      line,
		Multiline: true,
		StartByte: startByte,
		EndByte:   endByte,
		MultilineConfig: &scanner.MultilineCommentConfig{
			Start: []rune("/*"),
			End:   []rune("*/"),
		},
	}
}

func TestTagScanner(t *testing.T) {
	tests := []struct {
		name     string
		comments []*scanner.Comment
		expected []ReviewComment
		stderr   string
	}{
		{
			name:     "simple review message",
			comments: []*scanner.Comment{lineComment("// review: simple message", 1, 0, 25)},
			expected: []ReviewComment{
				{Tag: TagReview, Name: "", Message: "simple message", File: "test.go", Line: 1, EndLine: 1, StartByte: 0, EndByte: 25},
			},
		},
		{
			name:     "review with name",
			comments: []*scanner.Comment{lineComment("// review(auth): check token", 5, 10, 38)},
			expected: []ReviewComment{
				{Tag: TagReview, Name: "auth", Message: "check token", File: "test.go", Line: 5, EndLine: 5, StartByte: 10, EndByte: 38},
			},
		},
		{
			name:     "discuss with name",
			comments: []*scanner.Comment{lineComment("// discuss(auth): should we?", 3, 0, 28)},
			expected: []ReviewComment{
				{Tag: TagDiscuss, Name: "auth", Message: "should we?", File: "test.go", Line: 3, EndLine: 3, StartByte: 0, EndByte: 28},
			},
		},
		{
			name:     "explain without name",
			comments: []*scanner.Comment{lineComment("// explain: why this pattern", 1, 0, 28)},
			expected: []ReviewComment{
				{Tag: TagExplain, Name: "", Message: "why this pattern", File: "test.go", Line: 1, EndLine: 1, StartByte: 0, EndByte: 28},
			},
		},
		{
			name:     "see with name",
			comments: []*scanner.Comment{lineComment("// see: auth", 7, 0, 12)},
			expected: []ReviewComment{
				{Tag: TagSee, Name: "auth", Message: "", File: "test.go", Line: 7, EndLine: 7, StartByte: 0, EndByte: 12},
			},
		},
		{
			name:     "also with name",
			comments: []*scanner.Comment{lineComment("// also: auth", 8, 0, 13)},
			expected: []ReviewComment{
				{Tag: TagAlso, Name: "auth", Message: "", File: "test.go", Line: 8, EndLine: 8, StartByte: 0, EndByte: 13},
			},
		},
		{
			name:     "regular comment not matched",
			comments: []*scanner.Comment{lineComment("// regular comment", 1, 0, 18)},
			expected: nil,
		},
		{
			name:     "TODO not matched",
			comments: []*scanner.Comment{lineComment("// TODO: not our tag", 1, 0, 20)},
			expected: nil,
		},
		{
			name:     "block comment review",
			comments: []*scanner.Comment{blockComment("/* review(perf): block comment */", 1, 0, 32)},
			expected: []ReviewComment{
				{Tag: TagReview, Name: "perf", Message: "block comment", File: "test.go", Line: 1, EndLine: 1, StartByte: 0, EndByte: 32, Multiline: true},
			},
		},
		{
			name:     "case sensitivity - Review not matched",
			comments: []*scanner.Comment{lineComment("// Review: capitalized", 1, 0, 22)},
			expected: nil,
		},
		{
			name:     "review with hyphenated name",
			comments: []*scanner.Comment{lineComment("// review(auth-flow): multi-word message here", 1, 0, 45)},
			expected: []ReviewComment{
				{Tag: TagReview, Name: "auth-flow", Message: "multi-word message here", File: "test.go", Line: 1, EndLine: 1, StartByte: 0, EndByte: 45},
			},
		},
		{
			name:     "two-line block comment",
			comments: []*scanner.Comment{blockComment("/* review(fix):\n * detailed message */", 10, 0, 37)},
			expected: []ReviewComment{
				{Tag: TagReview, Name: "fix", Message: "detailed message", File: "test.go", Line: 10, EndLine: 11, StartByte: 0, EndByte: 37, Multiline: true},
			},
		},
		{
			name:     "three-line block comment",
			comments: []*scanner.Comment{blockComment("/* review(fix):\n * line two\n * line three */", 1, 0, 44)},
			expected: []ReviewComment{
				{Tag: TagReview, Name: "fix", Message: "line two line three", File: "test.go", Line: 1, EndLine: 3, StartByte: 0, EndByte: 44, Multiline: true},
			},
		},
		{
			name:     "block comment without star prefix",
			comments: []*scanner.Comment{blockComment("/* review(fix):\n   continuation without star */", 1, 0, 47)},
			expected: []ReviewComment{
				{Tag: TagReview, Name: "fix", Message: "continuation without star", File: "test.go", Line: 1, EndLine: 2, StartByte: 0, EndByte: 47, Multiline: true},
			},
		},
		{
			name:     "block comment with blank continuation",
			comments: []*scanner.Comment{blockComment("/* review(fix):\n *\n * after blank */", 1, 0, 35)},
			expected: []ReviewComment{
				{Tag: TagReview, Name: "fix", Message: " after blank", File: "test.go", Line: 1, EndLine: 3, StartByte: 0, EndByte: 35, Multiline: true},
			},
		},
		{
			name:     "see with no name - not matched",
			comments: []*scanner.Comment{lineComment("// see:", 1, 0, 7)},
			expected: nil,
		},
		{
			name:     "see with extra tokens",
			comments: []*scanner.Comment{lineComment("// see: auth stuff", 5, 0, 18)},
			expected: []ReviewComment{
				{Tag: TagSee, Name: "auth", Message: "", File: "test.go", Line: 5, EndLine: 5, StartByte: 0, EndByte: 18},
			},
			stderr: `warning: extra tokens after name in "see: auth stuff" at test.go:5 (using "auth")`,
		},
		{
			name:     "EndLine for line comment",
			comments: []*scanner.Comment{lineComment("// review: msg", 5, 0, 14)},
			expected: []ReviewComment{
				{Tag: TagReview, Name: "", Message: "msg", File: "test.go", Line: 5, EndLine: 5, StartByte: 0, EndByte: 14},
			},
		},
		{
			name:     "EndLine for block comment spanning 3 lines",
			comments: []*scanner.Comment{blockComment("/* review(x): msg\nline2\nline3 */", 10, 0, 32)},
			expected: []ReviewComment{
				{Tag: TagReview, Name: "x", Message: "msg line2 line3", File: "test.go", Line: 10, EndLine: 12, StartByte: 0, EndByte: 32, Multiline: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMock(tt.comments...)

			// Capture stderr for warning tests.
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			ts := NewTagScanner(mock, "test.go")

			var got []ReviewComment
			for ts.Scan() {
				got = append(got, *ts.Next())
			}

			if err := ts.Err(); err != nil {
				t.Fatalf("tag scanner error: %v", err)
			}

			_ = w.Close()
			os.Stderr = oldStderr

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			stderrOut := buf.String()

			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Errorf("review comments mismatch (-want +got):\n%s", diff)
			}

			if tt.stderr != "" && !containsStr(stderrOut, tt.stderr) {
				t.Errorf("expected stderr containing %q, got %q", tt.stderr, stderrOut)
			}
		})
	}
}

func TestTagScannerWithKnownTags(t *testing.T) {
	knownTags := map[string]struct{}{
		"review":  {},
		"discuss": {},
		"explain": {},
		"critique": {},
		"see":     {},
		"also":    {},
	}

	tests := []struct {
		name     string
		comments []*scanner.Comment
		expected []ReviewComment
	}{
		{
			name:     "extension tag critique accepted",
			comments: []*scanner.Comment{lineComment("// critique(perf): check allocation", 1, 0, 35)},
			expected: []ReviewComment{
				{Tag: "critique", Name: "perf", Message: "check allocation", File: "test.go", Line: 1, EndLine: 1, StartByte: 0, EndByte: 35},
			},
		},
		{
			name:     "unknown tag todo silently skipped",
			comments: []*scanner.Comment{lineComment("// todo: fix this later", 1, 0, 23)},
			expected: nil,
		},
		{
			name:     "unknown tag note silently skipped",
			comments: []*scanner.Comment{lineComment("// note: important thing", 1, 0, 24)},
			expected: nil,
		},
		{
			name:     "unknown tag fixme silently skipped",
			comments: []*scanner.Comment{lineComment("// fixme: broken thing", 1, 0, 22)},
			expected: nil,
		},
		{
			name:     "hyphenated tag name matched",
			comments: []*scanner.Comment{lineComment("// my-tag: something", 1, 0, 20)},
			expected: nil, // my-tag not in knownTags
		},
		{
			name:     "underscore tag name matched",
			comments: []*scanner.Comment{lineComment("// my_tag: something", 1, 0, 20)},
			expected: nil, // my_tag not in knownTags
		},
		{
			name:     "numeric start not matched",
			comments: []*scanner.Comment{lineComment("// 123invalid: something", 1, 0, 24)},
			expected: nil,
		},
		{
			name:     "pattern B see still works with knownTags",
			comments: []*scanner.Comment{lineComment("// see: auth", 1, 0, 12)},
			expected: []ReviewComment{
				{Tag: TagSee, Name: "auth", Message: "", File: "test.go", Line: 1, EndLine: 1, StartByte: 0, EndByte: 12},
			},
		},
		{
			name:     "pattern B priority - see: auth is cross-reference not generic tag",
			comments: []*scanner.Comment{lineComment("// see: auth", 1, 0, 12)},
			expected: []ReviewComment{
				{Tag: TagSee, Name: "auth", Message: "", File: "test.go", Line: 1, EndLine: 1, StartByte: 0, EndByte: 12},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := newMock(tt.comments...)
			ts := NewTagScannerWithTags(mock, "test.go", knownTags)

			var got []ReviewComment
			for ts.Scan() {
				got = append(got, *ts.Next())
			}

			if err := ts.Err(); err != nil {
				t.Fatalf("tag scanner error: %v", err)
			}

			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Errorf("review comments mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
