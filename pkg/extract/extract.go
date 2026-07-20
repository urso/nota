// Package extract handles extraction of review comments from source files.
package extract

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/urso/nota/pkg/blocks"
	"github.com/urso/nota/pkg/deleter"
	"github.com/urso/nota/pkg/git"
	"github.com/urso/nota/pkg/grouper"
	"github.com/urso/nota/pkg/parser"
	"github.com/urso/nota/pkg/scanner"
	"github.com/urso/nota/pkg/thread"
)

// Config holds the configuration for an extraction run.
type Config struct {
	Files        iter.Seq[string]
	KnownTags    map[string]struct{}
	Dir          string
	ContextLines int
}

// Run extracts review comments from source files and writes them as XML threads.
// It processes files, groups comments, writes thread files, and deletes comments from source.
func Run(cfg Config) error {
	comments, fileContents, fileRanges, _, filePerms, err := ProcessFiles(cfg.Files, cfg.KnownTags)
	if err != nil {
		return err
	}

	groups := grouper.GroupComments(comments, fileContents, cfg.ContextLines)

	if err := WriteThreadFiles(cfg.Dir, groups, fileContents); err != nil {
		return err
	}

	return DeleteFromFiles(fileContents, fileRanges, filePerms)
}

// fileResult holds the scan/parse output for a single file.
type fileResult struct {
	path     string
	comments []parser.ReviewComment
	contents []byte
	ranges   []deleter.ByteRange
	blocks   *blocks.File
	perm     os.FileMode
}

// ProcessFiles runs scan → parse for each file using parallel workers.
// Returns comments (with adjusted line numbers), file contents, byte ranges, blocks, and file permissions.
// When knownTags is non-nil, only tags in the set are accepted; otherwise all tags pass through.
func ProcessFiles(files iter.Seq[string], knownTags map[string]struct{}) ([]parser.ReviewComment, map[string][]byte, map[string][]deleter.ByteRange, map[string]*blocks.File, map[string]os.FileMode, error) {
	numWorkers := runtime.NumCPU()
	paths := make(chan string)
	results := make(chan fileResult)

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

	go func() {
		defer close(results)
		for filePath := range files {
			paths <- filePath
		}
		close(paths)
		wg.Wait()
	}()

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

		// Two-pointer walk: both comments and blocks are sorted by byte offset.
		blockIdx := 0
		for i := range comments {
			pos := ranges[i].Start
			for blockIdx < len(bf.Blocks) {
				b := bf.Blocks[blockIdx]
				if pos >= b.StartByte && pos < b.StartByte+int64(len(b.Content)) {
					comments[i].Line = bf.AdjustedLine(blockIdx)
					break
				}
				if pos < b.StartByte {
					break
				}
				blockIdx++
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

func expandByteRange(content []byte, start, end int64) deleter.ByteRange {
	if start < 0 {
		start = 0
	}
	if end > int64(len(content)) {
		end = int64(len(content))
	}
	if start > end {
		return deleter.ByteRange{Start: start, End: end}
	}

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

	lineEnd := end
	for lineEnd < int64(len(content)) && content[lineEnd] != '\n' {
		if content[lineEnd] != ' ' && content[lineEnd] != '\t' && content[lineEnd] != '\r' {
			break
		}
		lineEnd++
	}

	allWhitespaceAfter := lineEnd >= int64(len(content)) || content[lineEnd] == '\n'

	if allWhitespaceBefore && allWhitespaceAfter {
		start = lineStart
		end = lineEnd
		if end < int64(len(content)) && content[end] == '\n' {
			end++
		}
	} else if !allWhitespaceBefore {
		for start > lineStart && (content[start-1] == ' ' || content[start-1] == '\t') {
			start--
		}
	}

	return deleter.ByteRange{Start: start, End: end}
}

// DeleteFromFiles removes extracted comments from source files.
func DeleteFromFiles(fileContents map[string][]byte, fileRanges map[string][]deleter.ByteRange, filePerms map[string]os.FileMode) error {
	for filePath, ranges := range fileRanges {
		if len(ranges) == 0 {
			continue
		}

		content, ok := fileContents[filePath]
		if !ok {
			return fmt.Errorf("no content for %s", filePath)
		}

		perm, ok := filePerms[filePath]
		if !ok {
			perm = 0o644
		}

		modified, err := deleter.DeleteComments(content, ranges)
		if err != nil {
			return fmt.Errorf("deleting comments in %s: %w", filePath, err)
		}

		if err := deleter.WriteAtomic(filePath, modified, perm); err != nil {
			return fmt.Errorf("writing %s: %w", filePath, err)
		}
	}
	return nil
}

var reReviewFile = regexp.MustCompile(`^review-(\d+)\.(md|xml)$`)

// WriteThreadFiles writes groups as XML thread files in dir.
// Named groups go to {dir}/{name}.xml.
// Unnamed groups go to {dir}/review-{NNN}.xml.
// File paths are printed to stderr.
func WriteThreadFiles(dir string, groups []grouper.Group, fileContents map[string][]byte) error {
	return WriteThreadFilesTo(os.Stderr, dir, groups, fileContents)
}

// WriteThreadFilesTo writes groups as XML thread files in dir, logging paths to w.
func WriteThreadFilesTo(w io.Writer, dir string, groups []grouper.Group, fileContents map[string][]byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolving directory %s: %w", dir, err)
	}

	commit, err := git.HeadCommit("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not get HEAD commit: %v\n", err)
		commit = ""
	}

	// Pre-split file contents into lines once for all groups.
	fileLines := splitFileLines(fileContents)

	nextNum := nextReviewNumber(dir)

	for _, g := range groups {
		var filePath string
		if g.Name != "" {
			safeName, err := sanitizeGroupName(g.Name)
			if err != nil {
				return fmt.Errorf("invalid group name %q: %w", g.Name, err)
			}
			filePath = filepath.Join(dir, safeName+".xml")
		} else {
			filePath = filepath.Join(dir, fmt.Sprintf("review-%03d.xml", nextNum))
			nextNum++
		}

		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("resolving path %s: %w", filePath, err)
		}
		if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) {
			return fmt.Errorf("group name %q resolves outside target directory", g.Name)
		}

		t := groupToThread(g, commit, fileLines)
		if err := thread.WriteThread(filePath, t); err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\n", filePath)
	}

	return nil
}

func sanitizeGroupName(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty group name")
	}

	for _, c := range name {
		switch c {
		case '/', '\\':
			return "", fmt.Errorf("contains path separator")
		case '\x00':
			return "", fmt.Errorf("contains null byte")
		case ':', '<', '>', '|', '*', '?', '"':
			return "", fmt.Errorf("contains invalid filename character %q", c)
		}
	}

	if name == "." || name == ".." {
		return "", fmt.Errorf("invalid name %q", name)
	}

	return name, nil
}

func nextReviewNumber(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}

	max := 0
	for _, e := range entries {
		m := reReviewFile.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return max + 1
}
