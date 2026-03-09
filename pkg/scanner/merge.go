package scanner

import (
	"bytes"
	"slices"
)

// MergeLineComments wraps a CommentScanner and merges consecutive line comments
// into single multiline Comment blocks. Two line comments are merged if they
// use the same comment marker, appear on adjacent lines, and have the same
// indentation (leading whitespace before the comment marker).
//
// Merged comments have Multiline set to true, LineConfig preserved, and Text
// containing the joined lines separated by newlines (each still prefixed with
// the comment marker).
type MergeLineComments struct {
	inner   *CommentScanner
	content []byte // decoded file content for indentation checks

	current *Comment // result of last Scan(), returned by Next()
	buf     *Comment // look-ahead comment buffered during merging
	err     error
}

// NewMergeLineComments creates a new merging scanner.
// content is the decoded file content used to check indentation.
func NewMergeLineComments(inner *CommentScanner, content []byte) *MergeLineComments {
	return &MergeLineComments{
		inner:   inner,
		content: content,
	}
}

// Config delegates to the inner scanner.
func (m *MergeLineComments) Config() *Config {
	return m.inner.Config()
}

// Next returns the current merged comment.
func (m *MergeLineComments) Next() *Comment {
	return m.current
}

// Err returns the first error encountered.
func (m *MergeLineComments) Err() error {
	if m.err != nil {
		return m.err
	}
	return m.inner.Err()
}

// Scan advances to the next (possibly merged) comment.
func (m *MergeLineComments) Scan() bool {
	// Get the next comment: either from the look-ahead buffer or the inner scanner.
	var head *Comment
	if m.buf != nil {
		head = m.buf
		m.buf = nil
	} else {
		if !m.inner.Scan() {
			m.err = m.inner.Err()
			m.current = nil
			return false
		}
		head = m.copyComment(m.inner.Next())
	}

	// Only merge line comments (not block comments).
	if head.LineConfig == nil {
		m.current = head
		return true
	}

	// Try to absorb consecutive line comments with the same marker and indentation.
	headIndent := m.indentAt(head.StartByte)
	lastLine := head.Line

	for m.inner.Scan() {
		next := m.inner.Next()

		if next.LineConfig == nil ||
			!slices.Equal(next.LineConfig.Start, head.LineConfig.Start) ||
			next.Line != lastLine+1 ||
			!bytes.Equal(m.indentAt(next.StartByte), headIndent) {
			// Not a continuation — buffer for next Scan() call.
			m.buf = m.copyComment(next)
			break
		}

		// Merge: extend head to include next.
		head.Text += "\n" + next.Text
		head.EndByte = next.EndByte
		head.Multiline = true
		lastLine = next.Line
	}

	if m.inner.Err() != nil {
		m.err = m.inner.Err()
	}

	m.current = head
	return true
}

// indentAt returns the leading whitespace on the line containing byte offset pos.
func (m *MergeLineComments) indentAt(pos int64) []byte {
	lineStart := pos
	for lineStart > 0 && m.content[lineStart-1] != '\n' {
		lineStart--
	}
	end := lineStart
	for end < pos && (m.content[end] == ' ' || m.content[end] == '\t') {
		end++
	}
	return m.content[lineStart:end]
}

func (m *MergeLineComments) copyComment(c *Comment) *Comment {
	cp := *c
	return &cp
}
