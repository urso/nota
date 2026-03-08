package deleter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDeleteComments(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		ranges   []ByteRange
		expected string
		wantErr  bool
	}{
		{
			name:     "single range middle",
			content:  "hello world goodbye",
			ranges:   []ByteRange{{Start: 6, End: 12}},
			expected: "hello goodbye",
		},
		{
			name:     "multiple non-overlapping ranges",
			content:  "aaa bbb ccc ddd",
			ranges:   []ByteRange{{Start: 4, End: 8}, {Start: 12, End: 15}},
			expected: "aaa ccc ",
		},
		{
			name:     "range at start",
			content:  "// comment\ncode\n",
			ranges:   []ByteRange{{Start: 0, End: 11}},
			expected: "code\n",
		},
		{
			name:     "range at end",
			content:  "code\n// comment",
			ranges:   []ByteRange{{Start: 5, End: 15}},
			expected: "code\n",
		},
		{
			name:     "empty ranges",
			content:  "unchanged",
			ranges:   nil,
			expected: "unchanged",
		},
		{
			name:     "multi-byte UTF-8",
			content:  "héllo // comment\nworld",
			ranges:   []ByteRange{{Start: 7, End: 18}}, // "// comment\n" starts at byte 7, \n is byte 17, End=18
			expected: "héllo world",
		},
		{
			name:     "full line deletion including newline",
			content:  "line1\n    // review: msg\nline3\n",
			ranges:   []ByteRange{{Start: 6, End: 25}}, // includes leading spaces and trailing \n
			expected: "line1\nline3\n",
		},
		{
			name:     "partial line - trailing comment",
			content:  "code // review: msg\nmore\n",
			ranges:   []ByteRange{{Start: 5, End: 19}},
			expected: "code \nmore\n",
		},
		{
			name:    "overlapping ranges error",
			content: "hello world",
			ranges:  []ByteRange{{Start: 0, End: 6}, {Start: 3, End: 8}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DeleteComments([]byte(tt.content), tt.ranges)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(tt.expected, string(got)); diff != "" {
				t.Errorf("content mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	content := []byte("hello world\n")
	perm := os.FileMode(0o644)

	err := WriteAtomic(path, content, perm)
	if err != nil {
		t.Fatal(err)
	}

	// Verify content.
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(string(content), string(got)); diff != "" {
		t.Errorf("content mismatch (-want +got):\n%s", diff)
	}

	// Verify permissions.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != perm {
		t.Errorf("expected perm %o, got %o", perm, info.Mode().Perm())
	}
}
