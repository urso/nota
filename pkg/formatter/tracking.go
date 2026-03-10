package formatter

import (
	"fmt"
	"io"

	"github.com/urso/nota/pkg/grouper"
)

// FormatTracking writes a single group as a tracking file section.
// The output uses ## headings (not ###) and includes code context.
// This does NOT write the frontmatter — callers handle that separately.
func FormatTracking(w io.Writer, g grouper.Group) error {
	ew := &errWriter{w: w}

	for _, e := range g.Entries {
		ew.printf("## %s — %s:%d\n\n", e.Tag, e.File, e.Line)

		if e.Comment != "" {
			ew.printf("%s\n\n", e.Comment)
		}

		if hasContext(e.Context) {
			writeContextBlock(ew, e.Context, e.File)
		}
	}

	for _, r := range g.References {
		ew.printf("## see — %s:%d\n\n", r.File, r.Line)

		if hasContext(r.Context) {
			writeContextBlock(ew, r.Context, r.File)
		}
	}

	return ew.err
}

// FormatTrackingFrontmatter writes YAML frontmatter for a tracking file.
func FormatTrackingFrontmatter(w io.Writer, status, group string) error {
	ew := &errWriter{w: w}
	ew.println("---")
	ew.printf("status: %s\n", status)
	if group != "" {
		ew.printf("group: %s\n", group)
	}
	ew.println("---")
	ew.println()
	return ew.err
}

// FormatTrackingFile writes a complete tracking file (frontmatter + sections).
func FormatTrackingFile(w io.Writer, g grouper.Group) error {
	if err := FormatTrackingFrontmatter(w, "open", g.Name); err != nil {
		return fmt.Errorf("writing frontmatter: %w", err)
	}
	return FormatTracking(w, g)
}
