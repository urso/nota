package grouper

// ContextLines holds surrounding code context for a comment.
type ContextLines struct {
	Before []string
	After  []string
}

// extractContext returns surrounding context lines for a comment position.
// lines is the file content pre-split by newline.
// maxLines is the maximum lines to extract per side.
// Stops at blank lines (empty or whitespace-only).
func extractContext(lines [][]byte, startLine, endLine, maxLines int) ContextLines {
	if maxLines <= 0 {
		return ContextLines{}
	}

	// Convert 1-indexed lines to 0-indexed.
	startIdx := startLine - 1
	endIdx := endLine - 1

	var before []string
	for i := startIdx - 1; i >= 0 && len(before) < maxLines; i-- {
		line := string(lines[i])
		if isBlankLine(line) {
			break
		}
		before = append([]string{line}, before...)
	}

	var after []string
	for i := endIdx + 1; i < len(lines) && len(after) < maxLines; i++ {
		line := string(lines[i])
		if isBlankLine(line) {
			break
		}
		after = append(after, line)
	}

	return ContextLines{
		Before: before,
		After:  after,
	}
}

func isBlankLine(s string) bool {
	for _, c := range s {
		if c != ' ' && c != '\t' && c != '\r' {
			return false
		}
	}
	return true
}
