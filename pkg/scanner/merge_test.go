package scanner

import (
	"bytes"
	"testing"
)

type mergeExpected struct {
	text      string
	line      int
	multiline bool
}

func TestMergeLineComments(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		lang     string
		expected []mergeExpected
	}{
		{
			name:     "single line comment stays single",
			input:    "// hello\n",
			lang:     "Go",
			expected: []mergeExpected{{text: "// hello", line: 1, multiline: false}},
		},
		{
			name:     "two adjacent line comments merge",
			input:    "// first\n// second\n",
			lang:     "Go",
			expected: []mergeExpected{{text: "// first\n// second", line: 1, multiline: true}},
		},
		{
			name:     "three adjacent line comments merge",
			input:    "// a\n// b\n// c\n",
			lang:     "Go",
			expected: []mergeExpected{{text: "// a\n// b\n// c", line: 1, multiline: true}},
		},
		{
			name:  "blank line separates blocks",
			input: "// first\n\n// second\n",
			lang:  "Go",
			expected: []mergeExpected{
				{text: "// first", line: 1, multiline: false},
				{text: "// second", line: 3, multiline: false},
			},
		},
		{
			name:  "code between separates blocks",
			input: "// first\nvar x = 1\n// second\n",
			lang:  "Go",
			expected: []mergeExpected{
				{text: "// first", line: 1, multiline: false},
				{text: "// second", line: 3, multiline: false},
			},
		},
		{
			name:  "different indentation separates blocks",
			input: "    // indented\n// not indented\n",
			lang:  "Go",
			expected: []mergeExpected{
				{text: "// indented", line: 1, multiline: false},
				{text: "// not indented", line: 2, multiline: false},
			},
		},
		{
			name:     "same indentation merges",
			input:    "    // first\n    // second\n",
			lang:     "Go",
			expected: []mergeExpected{{text: "// first\n// second", line: 1, multiline: true}},
		},
		{
			name:  "block comment not merged with line comment",
			input: "/* block */\n// line\n",
			lang:  "Go",
			expected: []mergeExpected{
				{text: "/* block */", line: 1, multiline: true},
				{text: "// line", line: 2, multiline: false},
			},
		},
		{
			name:     "hash comments merge",
			input:    "# first\n# second\n",
			lang:     "Python",
			expected: []mergeExpected{{text: "# first\n# second", line: 1, multiline: true}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := []byte(tt.input)
			config, ok := LanguagesConfig[tt.lang]
			if !ok {
				t.Fatalf("no config for language %s", tt.lang)
			}

			inner := New(bytes.NewReader(content), config)
			merger := NewMergeLineComments(inner, content)

			var got []mergeExpected

			for merger.Scan() {
				c := merger.Next()
				got = append(got, mergeExpected{
					text:      c.Text,
					line:      c.Line,
					multiline: c.Multiline,
				})
			}

			if err := merger.Err(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d comments, got %d: %+v", len(tt.expected), len(got), got)
			}

			for i, exp := range tt.expected {
				if got[i].text != exp.text {
					t.Errorf("comment[%d] text = %q, want %q", i, got[i].text, exp.text)
				}
				if got[i].line != exp.line {
					t.Errorf("comment[%d] line = %d, want %d", i, got[i].line, exp.line)
				}
				if got[i].multiline != exp.multiline {
					t.Errorf("comment[%d] multiline = %v, want %v", i, got[i].multiline, exp.multiline)
				}
			}
		})
	}
}

func TestMergeLineCommentsByteRange(t *testing.T) {
	content := []byte("    // first\n    // second\n    // third\n")
	config := LanguagesConfig["Go"]
	inner := New(bytes.NewReader(content), config)
	merger := NewMergeLineComments(inner, content)

	if !merger.Scan() {
		t.Fatal("expected a comment")
	}

	c := merger.Next()
	if c.StartByte != 4 {
		t.Errorf("StartByte = %d, want 4", c.StartByte)
	}
	// EndByte should be at the end of "// third" on the last line (before newline).
	// "    // first\n    // second\n    // third" = 4 + 8 + 1 + 4 + 9 + 1 + 4 + 8 = 39
	expectedEnd := int64(len("    // first\n    // second\n    // third"))
	if c.EndByte != expectedEnd {
		t.Errorf("EndByte = %d, want %d", c.EndByte, expectedEnd)
	}

	if merger.Scan() {
		t.Error("expected no more comments")
	}
}
