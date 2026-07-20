package extract

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/urso/nota/pkg/deleter"
)

func TestExpandByteRange(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		start    int64
		end      int64
		expected deleter.ByteRange
	}{
		{
			name:     "comment-only line",
			content:  "line1\n    // review: msg\nline3\n",
			start:    10,
			end:      24,
			expected: deleter.ByteRange{Start: 6, End: 25},
		},
		{
			name:     "trailing comment",
			content:  "code // review: msg\nmore\n",
			start:    5,
			end:      19,
			expected: deleter.ByteRange{Start: 4, End: 19},
		},
		{
			name:     "comment at file start",
			content:  "// review: msg\ncode\n",
			start:    0,
			end:      14,
			expected: deleter.ByteRange{Start: 0, End: 15},
		},
		{
			name:     "comment at file end no newline",
			content:  "code\n// review: msg",
			start:    5,
			end:      19,
			expected: deleter.ByteRange{Start: 5, End: 19},
		},
		{
			name:     "tab indented comment",
			content:  "code\n\t\t// review: msg\nmore\n",
			start:    7,
			end:      21,
			expected: deleter.ByteRange{Start: 5, End: 22},
		},
		{
			name:     "negative start clamped",
			content:  "// review: msg\n",
			start:    -5,
			end:      14,
			expected: deleter.ByteRange{Start: 0, End: 15},
		},
		{
			name:     "oversized end clamped",
			content:  "// review: msg",
			start:    0,
			end:      100,
			expected: deleter.ByteRange{Start: 0, End: 14},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandByteRange([]byte(tt.content), tt.start, tt.end)
			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
