package scanner

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestScannerByteOffsets(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		config   *Config
		expected []Comment
	}{
		{
			name:   "Go line comment",
			input:  "x := 1\n// review(auth): check token\ny := 2\n",
			config: LanguagesConfig["Go"],
			expected: []Comment{
				{
					Text:      "// review(auth): check token",
					Line:      2,
					Multiline: false,
					StartByte: 7,
					EndByte:   35,
				},
			},
		},
		{
			name:   "Python hash comment",
			input:  "x = 1\n# review: message\ny = 2\n",
			config: LanguagesConfig["Python"],
			expected: []Comment{
				{
					Text:      "# review: message",
					Line:      2,
					Multiline: false,
					StartByte: 6,
					EndByte:   23,
				},
			},
		},
		{
			name:   "C block comment",
			input:  "int x;\n/* review(fix): block */\nint y;\n",
			config: LanguagesConfig["C"],
			expected: []Comment{
				{
					Text:      "/* review(fix): block */",
					Line:      2,
					Multiline: true,
					StartByte: 7,
					EndByte:   31,
				},
			},
		},
		{
			name:   "comment inside string is ignored",
			input:  "s := \"// review: inside string\"\n// real comment\n",
			config: LanguagesConfig["Go"],
			expected: []Comment{
				{
					Text:      "// real comment",
					Line:      2,
					Multiline: false,
					StartByte: 32,
					EndByte:   47,
				},
			},
		},
		{
			// x := "héllo"\n = x(1) (1) :(1) =(1) (1) "(1) h(1) é(2) l(1) l(1) o(1) "(1) \n(1) = 14 bytes
			name:   "multi-byte UTF-8 before comment",
			input:  "x := \"h\u00e9llo\"\n// after utf8\n",
			config: LanguagesConfig["Go"],
			expected: []Comment{
				{
					Text:      "// after utf8",
					Line:      2,
					Multiline: false,
					StartByte: 14,
					EndByte:   27,
				},
			},
		},
		{
			name:   "multiple comments",
			input:  "// first\n// second\n",
			config: LanguagesConfig["Go"],
			expected: []Comment{
				{
					Text:      "// first",
					Line:      1,
					Multiline: false,
					StartByte: 0,
					EndByte:   8,
				},
				{
					Text:      "// second",
					Line:      2,
					Multiline: false,
					StartByte: 9,
					EndByte:   18,
				},
			},
		},
		{
			name:   "multiline block comment spans lines",
			input:  "/* line1\nline2\nline3 */\n",
			config: LanguagesConfig["Go"],
			expected: []Comment{
				{
					Text:      "/* line1\nline2\nline3 */",
					Line:      1,
					Multiline: true,
					StartByte: 0,
					EndByte:   23,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(strings.NewReader(tt.input), tt.config)

			var got []Comment
			for s.Scan() {
				c := s.Next()
				got = append(got, Comment{
					Text:      c.Text,
					Line:      c.Line,
					Multiline: c.Multiline,
					StartByte: c.StartByte,
					EndByte:   c.EndByte,
				})
			}

			if err := s.Err(); err != nil {
				t.Fatalf("scanner error: %v", err)
			}

			if diff := cmp.Diff(tt.expected, got); diff != "" {
				t.Errorf("comments mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
