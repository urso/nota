package main

import (
	"errors"
	"fmt"
	"iter"
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/urso/nota/pkg/blocks"
	"github.com/urso/nota/pkg/deleter"
	"github.com/urso/nota/pkg/parser"
	"github.com/urso/nota/pkg/scanner"
)

// fileResult holds the scan/parse output for a single file.
type fileResult struct {
	path     string
	comments []parser.ReviewComment
	contents []byte
	ranges   []deleter.ByteRange
	blocks   *blocks.File
	perm     os.FileMode
}

// processFiles runs scan → parse for each file using parallel workers.
// Returns comments (with adjusted line numbers), file contents, byte ranges, blocks, and file permissions.
// When knownTags is non-nil, only tags in the set are accepted; otherwise all tags pass through.
func processFiles(files iter.Seq[string], knownTags map[string]struct{}) ([]parser.ReviewComment, map[string][]byte, map[string][]deleter.ByteRange, map[string]*blocks.File, map[string]os.FileMode, error) {
	numWorkers := runtime.NumCPU()
	paths := make(chan string)
	results := make(chan fileResult)

	// Fan out: N workers read and process files.
	var wg sync.WaitGroup
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range paths {
				if r, ok := processFile(filePath, knownTags); ok {
					results <- r
				}
			}
		}()
	}

	// Send file paths to workers, then wait for completion.
	go func() {
		defer close(results)
		for filePath := range files {
			paths <- filePath
		}
		close(paths)
		wg.Wait()
	}()

	// Collect results.
	var allComments []parser.ReviewComment
	fileContents := make(map[string][]byte)
	fileRanges := make(map[string][]deleter.ByteRange)
	fileBlocks := make(map[string]*blocks.File)
	filePerms := make(map[string]os.FileMode)

	for r := range results {
		allComments = append(allComments, r.comments...)
		fileContents[r.path] = r.contents
		if len(r.ranges) > 0 {
			fileRanges[r.path] = r.ranges
		}
		if r.blocks != nil {
			fileBlocks[r.path] = r.blocks
		}
		filePerms[r.path] = r.perm
	}

	return allComments, fileContents, fileRanges, fileBlocks, filePerms, nil
}

// processFile reads, scans, and parses a single file.
// Returns the result and true if the file produced output, or false to skip.
func processFile(filePath string, knownTags map[string]struct{}) (fileResult, bool) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot read %s: %v\n", filePath, err)
		return fileResult{}, false
	}

	info, err := os.Stat(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot stat %s: %v\n", filePath, err)
		return fileResult{}, false
	}

	result, err := scanner.FromBytes(filePath, content, "", "detect")
	if err != nil {
		if errors.Is(err, scanner.ErrUnsupportedLanguage) || errors.Is(err, scanner.ErrBinaryFile) {
			return fileResult{}, false
		}
		fmt.Fprintf(os.Stderr, "warning: cannot scan %s: %v\n", filePath, err)
		return fileResult{}, false
	}

	merged := scanner.NewMergeLineComments(result.Scanner, result.DecodedContents)
	var ts *parser.TagScanner
	if knownTags != nil {
		ts = parser.NewTagScannerWithTags(merged, filePath, knownTags)
	} else {
		ts = parser.NewTagScanner(merged, filePath)
	}

	var comments []parser.ReviewComment
	var ranges []deleter.ByteRange
	for ts.Scan() {
		rc := ts.Next()
		comments = append(comments, *rc)
		ranges = append(ranges, expandByteRange(result.DecodedContents, rc.StartByte, rc.EndByte))
	}

	if err := ts.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: parse error in %s: %v\n", filePath, err)
	}

	// Build blocks file and compute adjusted line numbers.
	var bf *blocks.File
	if len(ranges) > 0 {
		blockRanges := make([]blocks.Range, len(ranges))
		for i, r := range ranges {
			blockRanges[i] = blocks.Range{Start: r.Start, End: r.End}
		}
		sort.Slice(blockRanges, func(i, j int) bool {
			return blockRanges[i].Start < blockRanges[j].Start
		})
		bf = blocks.FromBytesAndRanges(result.DecodedContents, blockRanges)

		// Adjust line numbers for each comment.
		for i := range comments {
			blockIdx := findBlockForByte(bf, ranges[i].Start)
			if blockIdx >= 0 {
				comments[i].Line = bf.AdjustedLine(blockIdx)
			}
		}
	}

	return fileResult{
		path:     filePath,
		comments: comments,
		contents: result.DecodedContents,
		ranges:   ranges,
		blocks:   bf,
		perm:     info.Mode().Perm(),
	}, true
}

// findBlockForByte returns the block index containing the given byte offset.
func findBlockForByte(f *blocks.File, pos int64) int {
	for i, b := range f.Blocks {
		if pos >= b.StartByte && pos < b.StartByte+int64(len(b.Content)) {
			return i
		}
	}
	return -1
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

	// Scan backward: find start of line and check if prefix is all whitespace.
	lineStart := start
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}

	allWhitespaceBefore := true
	for i := lineStart; i < start; i++ {
		if content[i] != ' ' && content[i] != '\t' {
			allWhitespaceBefore = false
			break
		}
	}

	// Scan forward: skip whitespace after End until newline or non-whitespace.
	lineEnd := end
	for lineEnd < int64(len(content)) && content[lineEnd] != '\n' {
		if content[lineEnd] != ' ' && content[lineEnd] != '\t' && content[lineEnd] != '\r' {
			break
		}
		lineEnd++
	}

	// The forward scan only advanced past whitespace, so if we reached
	// a newline or EOF, everything after the comment is whitespace.
	allWhitespaceAfter := lineEnd >= int64(len(content)) || content[lineEnd] == '\n'

	if allWhitespaceBefore && allWhitespaceAfter {
		// Comment-only line: delete from start of line through newline.
		start = lineStart
		end = lineEnd
		if end < int64(len(content)) && content[end] == '\n' {
			end++
		}
	} else if !allWhitespaceBefore {
		// Trailing comment: trim whitespace between code and comment marker.
		for start > lineStart && (content[start-1] == ' ' || content[start-1] == '\t') {
			start--
		}
	}

	return deleter.ByteRange{Start: start, End: end}
}
