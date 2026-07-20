package formatter

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/urso/nota/pkg/grouper"
)

// extensionToLang maps file extensions to markdown code fence language tags.
var extensionToLang = map[string]string{
	".go":    "go",
	".py":    "python",
	".js":    "javascript",
	".ts":    "typescript",
	".rs":    "rust",
	".rb":    "ruby",
	".java":  "java",
	".c":     "c",
	".cpp":   "cpp",
	".h":     "c",
	".cs":    "csharp",
	".swift": "swift",
	".kt":    "kotlin",
	".sh":    "bash",
	".yaml":  "yaml",
	".yml":   "yaml",
	".json":  "json",
	".sql":   "sql",
	".html":  "html",
	".css":   "css",
	".php":   "php",
	".lua":   "lua",
	".r":     "r",
	".scala": "scala",
}

func langFromFile(file string) string {
	ext := strings.ToLower(filepath.Ext(file))
	if lang, ok := extensionToLang[ext]; ok {
		return lang
	}
	return ""
}

// errWriter wraps an io.Writer and captures the first error.
type errWriter struct {
	w   io.Writer
	err error
}

func (ew *errWriter) printf(format string, args ...interface{}) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintf(ew.w, format, args...)
}

func (ew *errWriter) println(args ...interface{}) {
	if ew.err != nil {
		return
	}
	_, ew.err = fmt.Fprintln(ew.w, args...)
}

// FormatMarkdown writes grouped review comments as markdown to w.
func FormatMarkdown(w io.Writer, groups []grouper.Group) error {
	ew := &errWriter{w: w}

	for i, g := range groups {
		if i > 0 {
			ew.println()
		}

		name := g.Name
		if name == "" {
			name = "(unnamed)"
		}
		ew.printf("## %s\n\n", name)

		for _, e := range g.Entries {
			ew.printf("### %s — %s:%d\n\n", e.Tag, e.File, e.Line)

			if e.Comment != "" {
				ew.printf("%s\n\n", e.Comment)
			}

			if hasContext(e.Context) {
				writeContextBlock(ew, e.Context, e.File)
			}
		}

		for _, r := range g.References {
			ew.printf("### see — %s:%d\n\n", r.File, r.Line)

			if hasContext(r.Context) {
				writeContextBlock(ew, r.Context, r.File)
			}
		}
	}

	return ew.err
}

func hasContext(ctx grouper.ContextLines) bool {
	return len(ctx.Before) > 0 || len(ctx.After) > 0
}

func writeContextBlock(ew *errWriter, ctx grouper.ContextLines, file string) {
	lang := langFromFile(file)
	// F13: Use 4-backtick fence to avoid collision with source containing triple backticks.
	ew.printf("````%s\n", lang)

	for _, line := range ctx.Before {
		ew.println(line)
	}

	ew.println(">>> comment <<<")

	for _, line := range ctx.After {
		ew.println(line)
	}

	ew.println("````")
	ew.println()
}
