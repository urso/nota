package formatter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/urso/aireview/pkg/grouper"
	"github.com/urso/aireview/pkg/parser"
	"gopkg.in/yaml.v3"
)

func TestFormatMarkdown(t *testing.T) {
	t.Run("single named group", func(t *testing.T) {
		groups := []grouper.Group{
			{
				Name: "auth",
				Entries: []grouper.Entry{
					{
						Tag:     parser.TagReview,
						File:    "src/auth.go",
						Line:    42,
						Comment: "check token expiry",
						Context: grouper.ContextLines{
							Before: []string{"func validate() {"},
							After:  []string{"\treturn nil"},
						},
					},
				},
			},
		}

		var buf bytes.Buffer
		err := FormatMarkdown(&buf, groups)
		if err != nil {
			t.Fatal(err)
		}

		out := buf.String()
		if !strings.Contains(out, "## auth") {
			t.Error("missing group heading")
		}
		if !strings.Contains(out, "### review — src/auth.go:42") {
			t.Error("missing entry heading")
		}
		if !strings.Contains(out, "check token expiry") {
			t.Error("missing comment text")
		}
		if !strings.Contains(out, "```go") {
			t.Error("missing code fence with language")
		}
		if !strings.Contains(out, ">>> comment <<<") {
			t.Error("missing comment marker")
		}
	})

	t.Run("named group with entry and reference", func(t *testing.T) {
		groups := []grouper.Group{
			{
				Name: "api",
				Entries: []grouper.Entry{
					{Tag: parser.TagReview, File: "api.go", Line: 10, Comment: "review this"},
				},
				References: []grouper.Reference{
					{
						File: "client.go",
						Line: 20,
						Context: grouper.ContextLines{
							Before: []string{"func call() {"},
						},
					},
				},
			},
		}

		var buf bytes.Buffer
		err := FormatMarkdown(&buf, groups)
		if err != nil {
			t.Fatal(err)
		}

		out := buf.String()
		if !strings.Contains(out, "### see — client.go:20") {
			t.Error("missing reference heading")
		}
	})

	t.Run("unnamed group", func(t *testing.T) {
		groups := []grouper.Group{
			{
				Name: "",
				Entries: []grouper.Entry{
					{Tag: parser.TagReview, File: "main.go", Line: 1, Comment: "unnamed"},
				},
			},
		}

		var buf bytes.Buffer
		err := FormatMarkdown(&buf, groups)
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(buf.String(), "## (unnamed)") {
			t.Error("missing unnamed heading")
		}
	})

	t.Run("empty groups", func(t *testing.T) {
		var buf bytes.Buffer
		err := FormatMarkdown(&buf, nil)
		if err != nil {
			t.Fatal(err)
		}
		if buf.Len() != 0 {
			t.Errorf("expected empty output, got %q", buf.String())
		}
	})
}

func TestFormatYAML(t *testing.T) {
	t.Run("roundtrip", func(t *testing.T) {
		groups := []grouper.Group{
			{
				Name: "auth",
				Entries: []grouper.Entry{
					{
						Tag:     parser.TagReview,
						File:    "src/auth.go",
						Line:    42,
						Comment: "Token expiry check",
						Context: grouper.ContextLines{
							Before: []string{"line1"},
							After:  []string{"line2"},
						},
					},
				},
				References: []grouper.Reference{
					{
						File: "src/token.go",
						Line: 7,
						Context: grouper.ContextLines{
							Before: []string{"before"},
							After:  []string{"after"},
						},
					},
				},
			},
		}

		var buf bytes.Buffer
		err := FormatYAML(&buf, groups)
		if err != nil {
			t.Fatal(err)
		}

		// Unmarshal and verify structure.
		var parsed []yamlGroup
		if err := yaml.Unmarshal(buf.Bytes(), &parsed); err != nil {
			t.Fatalf("invalid YAML: %v\n%s", err, buf.String())
		}

		if len(parsed) != 1 {
			t.Fatalf("expected 1 group, got %d", len(parsed))
		}
		if parsed[0].Name != "auth" {
			t.Errorf("expected name 'auth', got %q", parsed[0].Name)
		}
		if len(parsed[0].Entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(parsed[0].Entries))
		}
		if len(parsed[0].References) != 1 {
			t.Errorf("expected 1 reference, got %d", len(parsed[0].References))
		}
		if parsed[0].Entries[0].Comment != "Token expiry check" {
			t.Errorf("expected comment 'Token expiry check', got %q", parsed[0].Entries[0].Comment)
		}
	})

	t.Run("empty groups", func(t *testing.T) {
		var buf bytes.Buffer
		err := FormatYAML(&buf, nil)
		if err != nil {
			t.Fatal(err)
		}
	})
}
