package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/urso/aireview/pkg/deleter"
	"github.com/urso/aireview/pkg/parser"
	"github.com/urso/aireview/pkg/scanner"
)

// processFiles runs scan → parse for each file, returns comments, file contents, and byte ranges.
// The returned fileContents contains the decoded (UTF-8) content that matches the scanner's byte offsets.
// The returned filePerms contains the original file permissions for atomic writes.
func processFiles(files []string) ([]parser.ReviewComment, map[string][]byte, map[string][]deleter.ByteRange, map[string]os.FileMode, error) {
	var allComments []parser.ReviewComment
	fileContents := make(map[string][]byte)
	fileRanges := make(map[string][]deleter.ByteRange)
	filePerms := make(map[string]os.FileMode)

	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot read %s: %v\n", filePath, err)
			continue
		}

		info, err := os.Stat(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot stat %s: %v\n", filePath, err)
			continue
		}
		filePerms[filePath] = info.Mode().Perm()

		result, err := scanner.FromBytes(filePath, content, "", "detect")
		if err != nil {
			if errors.Is(err, scanner.ErrUnsupportedLanguage) || errors.Is(err, scanner.ErrBinaryFile) {
				continue
			}
			fmt.Fprintf(os.Stderr, "warning: cannot scan %s: %v\n", filePath, err)
			continue
		}

		// Use the decoded content for all byte-offset operations.
		// The scanner's byte offsets refer to positions in decoded content.
		fileContents[filePath] = result.DecodedContents

		ts := parser.NewTagScanner(result.Scanner, filePath)

		for ts.Scan() {
			rc := ts.Next()
			allComments = append(allComments, *rc)

			br := expandByteRange(result.DecodedContents, rc.StartByte, rc.EndByte)
			fileRanges[filePath] = append(fileRanges[filePath], br)
		}

		if err := ts.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: parse error in %s: %v\n", filePath, err)
		}
	}

	return allComments, fileContents, fileRanges, filePerms, nil
}

// expandByteRange expands a comment byte range for whole-line deletion.
func expandByteRange(content []byte, start, end int64) deleter.ByteRange {
	// F6: bounds validation
	if start < 0 {
		start = 0
	}
	if end > int64(len(content)) {
		end = int64(len(content))
	}
	if start > end {
		return deleter.ByteRange{Start: start, End: end}
	}

	s := start
	e := end

	// Scan backward: check if all bytes from start-of-line to Start are whitespace.
	lineStart := s
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}

	allWhitespaceBefore := true
	for i := lineStart; i < s; i++ {
		if content[i] != ' ' && content[i] != '\t' {
			allWhitespaceBefore = false
			break
		}
	}

	// Scan forward: check if bytes after End are whitespace followed by newline or EOF.
	lineEnd := e
	for lineEnd < int64(len(content)) && content[lineEnd] != '\n' {
		if content[lineEnd] != ' ' && content[lineEnd] != '\t' && content[lineEnd] != '\r' {
			break
		}
		lineEnd++
	}

	allWhitespaceAfter := true
	for i := e; i < lineEnd; i++ {
		if content[i] != ' ' && content[i] != '\t' && content[i] != '\r' {
			allWhitespaceAfter = false
			break
		}
	}

	if allWhitespaceBefore && allWhitespaceAfter {
		// Comment-only line: delete from start of line through newline.
		s = lineStart
		e = lineEnd
		if e < int64(len(content)) && content[e] == '\n' {
			e++
		}
	} else if !allWhitespaceBefore {
		// Trailing comment: trim whitespace between code and comment marker.
		for s > lineStart && (content[s-1] == ' ' || content[s-1] == '\t') {
			s--
		}
	}

	return deleter.ByteRange{Start: s, End: e}
}
