package parser

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/urso/aireview/pkg/scanner"
)

// Tag represents the type of review comment.
type Tag string

const (
	TagReview  Tag = "review"
	TagDiscuss Tag = "discuss"
	TagExplain Tag = "explain"
	TagSee     Tag = "see"
	TagAlso    Tag = "also"
)

// ReviewComment is a parsed review comment from source code.
type ReviewComment struct {
	Tag       Tag
	Name      string
	Message   string
	File      string
	Line      int
	EndLine   int
	StartByte int64
	EndByte   int64
	Multiline bool
}

// CommentScanner is the interface for scanning comments from source code.
type CommentScanner interface {
	Config() *scanner.Config
	Scan() bool
	Next() *scanner.Comment
	Err() error
}

var (
	// patternA matches review(name): message or review: message
	patternA = regexp.MustCompile(`(?:^|\s)(review|discuss|explain)(?:\(([^)]*)\))?\s*:\s*(.*)`)

	// patternB matches see: name or also: name
	patternB = regexp.MustCompile(`(?:^|\s)(see|also)\s*:\s*(\S+)(.*)`)
)

// TagScanner wraps a CommentScanner and extracts review tags.
type TagScanner struct {
	scanner  CommentScanner
	filePath string
	next     *ReviewComment
	err      error
}

// NewTagScanner creates a new TagScanner.
func NewTagScanner(s CommentScanner, filePath string) *TagScanner {
	return &TagScanner{
		scanner:  s,
		filePath: filePath,
	}
}

// Next returns the next ReviewComment.
func (t *TagScanner) Next() *ReviewComment {
	return t.next
}

// Err returns an error if one occurred.
func (t *TagScanner) Err() error {
	return t.err
}

// Scan advances to the next review comment. Returns false when done.
func (t *TagScanner) Scan() bool {
	for t.scanner.Scan() {
		comment := t.scanner.Next()
		rc := t.parseComment(comment)
		if rc != nil {
			t.next = rc
			return true
		}
	}

	t.err = t.scanner.Err()
	return false
}

// stripCommentMarker strips the line comment marker prefix (e.g. "// " or "# ") from text.
func stripCommentMarker(text string, c *scanner.Comment) string {
	if c.LineConfig != nil {
		marker := string(c.LineConfig.Start)
		text = strings.TrimPrefix(text, marker)
	}
	return text
}

func (t *TagScanner) parseComment(c *scanner.Comment) *ReviewComment {
	text := c.Text

	// For multiline comments, strip the start/end markers to get inner text.
	// For line comments, keep the full text (marker included) for regex matching.
	var innerText string
	var continuationLines []string

	if c.Multiline && c.MultilineConfig != nil {
		startMarker := string(c.MultilineConfig.Start)
		endMarker := string(c.MultilineConfig.End)
		innerText = strings.TrimPrefix(text, startMarker)
		innerText = strings.TrimSuffix(innerText, endMarker)
		innerText = strings.TrimSpace(innerText)

		// Split into first line and continuation for multiline.
		lines := strings.SplitN(innerText, "\n", 2)
		innerText = lines[0]
		if len(lines) > 1 {
			continuationLines = strings.Split(lines[1], "\n")
		}
	} else {
		// F10: Strip line comment marker before matching so "//review:" works like "// review:".
		innerText = stripCommentMarker(text, c)
	}

	// Compute EndLine.
	endLine := c.Line
	if c.Multiline {
		endLine = c.Line + strings.Count(c.Text, "\n")
	}

	// Try pattern A (review/discuss/explain).
	if m := patternA.FindStringSubmatch(innerText); m != nil {
		tag := Tag(m[1])
		name := m[2]
		message := strings.TrimSpace(m[3])

		// For multiline block comments, append continuation lines.
		if c.Multiline && len(continuationLines) > 0 {
			message = appendContinuation(message, continuationLines)
		}

		return &ReviewComment{
			Tag:       tag,
			Name:      name,
			Message:   message,
			File:      t.filePath,
			Line:      c.Line,
			EndLine:   endLine,
			StartByte: c.StartByte,
			EndByte:   c.EndByte,
			Multiline: c.Multiline,
		}
	}

	// Try pattern B (see/also).
	if m := patternB.FindStringSubmatch(innerText); m != nil {
		tag := Tag(m[1])
		name := m[2]
		extra := strings.TrimSpace(m[3])

		if extra != "" {
			fmt.Fprintf(os.Stderr, "warning: extra tokens after name in %q at %s:%d (using %q)\n",
				strings.TrimSpace(innerText), t.filePath, c.Line, name)
		}

		return &ReviewComment{
			Tag:       tag,
			Name:      name,
			Message:   "",
			File:      t.filePath,
			Line:      c.Line,
			EndLine:   endLine,
			StartByte: c.StartByte,
			EndByte:   c.EndByte,
			Multiline: c.Multiline,
		}
	}

	return nil
}

// appendContinuation joins continuation lines, stripping leading * prefixes.
func appendContinuation(initialMessage string, lines []string) string {
	var parts []string
	if initialMessage != "" {
		parts = append(parts, initialMessage)
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Strip leading * prefix common in block comments.
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)

		if line == "" {
			// Blank continuation line preserved as space.
			parts = append(parts, "")
			continue
		}

		parts = append(parts, line)
	}

	return strings.Join(parts, " ")
}
