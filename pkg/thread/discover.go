package thread

import (
	"encoding/xml"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ThreadFilter specifies criteria for filtering threads.
type ThreadFilter struct {
	Status string
	Goal   string
	Group  string
	Tag    string

	// Relationship queries - these require loading all threads first
	RefsOf       string // Return threads that this thread references
	DepsOf       string // Return threads that this thread depends on
	ReferencedBy string // Return threads that reference this thread
	BlockedBy    string // Return threads that depend on this thread
}

// ThreadInfo holds a thread and its source file path.
type ThreadInfo struct {
	Thread *Thread
	Path   string
}

// ListThreads scans dir for .xml files and returns threads matching the filter.
// Returns an error if any thread file cannot be parsed.
func ListThreads(dir string, filter ThreadFilter) ([]ThreadInfo, error) {
	// Handle relationship queries
	if filter.RefsOf != "" || filter.DepsOf != "" {
		return listRelatedTo(dir, filter)
	}
	if filter.ReferencedBy != "" || filter.BlockedBy != "" {
		return listRelatedFrom(dir, filter)
	}

	// Standard filtering via iterator
	var results []ThreadInfo
	for info, err := range AllThreads(dir) {
		if err != nil {
			return nil, err
		}
		if matchesFilter(info.Thread, filter) {
			results = append(results, info)
		}
	}
	return results, nil
}

// listRelatedTo finds threads that the source thread references or depends on.
func listRelatedTo(dir string, filter ThreadFilter) ([]ThreadInfo, error) {
	sourceID := filter.RefsOf
	if sourceID == "" {
		sourceID = filter.DepsOf
	}

	// Find the source thread
	source, err := FindThread(dir, sourceID)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, fmt.Errorf("thread not found: %s", sourceID)
	}

	// Collect target IDs
	var targetIDs []string
	if filter.RefsOf != "" {
		for _, ref := range source.Thread.Refs {
			if ref.Thread != "" {
				targetIDs = append(targetIDs, ref.Thread)
			}
		}
	}
	if filter.DepsOf != "" {
		for dep := range strings.SplitSeq(source.Thread.DependsOn, ",") {
			if d := strings.TrimSpace(dep); d != "" {
				targetIDs = append(targetIDs, d)
			}
		}
	}

	if len(targetIDs) == 0 {
		return nil, nil
	}

	// Find matching threads
	targetSet := make(map[string]bool, len(targetIDs))
	for _, id := range targetIDs {
		targetSet[id] = true
	}

	var results []ThreadInfo
	for info, err := range AllThreads(dir) {
		if err != nil {
			return nil, err
		}
		if targetSet[info.Thread.ID] {
			results = append(results, info)
		}
	}
	return results, nil
}

// listRelatedFrom finds threads that reference or depend on the target thread.
func listRelatedFrom(dir string, filter ThreadFilter) ([]ThreadInfo, error) {
	var results []ThreadInfo
	for info, err := range AllThreads(dir) {
		if err != nil {
			return nil, err
		}
		if filter.ReferencedBy != "" && referencesThread(info.Thread, filter.ReferencedBy) {
			results = append(results, info)
		}
		if filter.BlockedBy != "" && dependsOnThread(info.Thread, filter.BlockedBy) {
			results = append(results, info)
		}
	}
	return results, nil
}

// FindThread finds a thread by ID or number in the given directory.
// Accepts: full ID ("l:abc123..."), ID suffix ("abc123..."), or number ("16", "016").
// Returns nil if not found.
func FindThread(dir, query string) (*ThreadInfo, error) {
	// Check if query is a number
	var num int
	var isNum bool
	if n, err := strconv.Atoi(strings.TrimLeft(query, "0")); err == nil && n > 0 {
		num = n
		isNum = true
	}

	// Normalize ID query (add "l:" prefix if missing)
	idQuery := query
	if !isNum && !strings.HasPrefix(query, "l:") && !strings.HasPrefix(query, "gh:") {
		idQuery = "l:" + query
	}

	for info, err := range AllThreads(dir) {
		if err != nil {
			return nil, err
		}
		if isNum && info.Thread.Number == num {
			return &info, nil
		}
		if !isNum && info.Thread.ID == idQuery {
			return &info, nil
		}
	}
	return nil, nil
}

func matchesFilter(t *Thread, f ThreadFilter) bool {
	if f.Status != "" && t.Status != f.Status {
		return false
	}
	if f.Goal != "" && t.Goal != f.Goal {
		return false
	}
	if f.Group != "" && t.Group != f.Group {
		return false
	}
	if f.Tag != "" && !hasTag(t.Tags, f.Tag) {
		return false
	}
	return true
}

func hasTag(tags, tag string) bool {
	for t := range strings.SplitSeq(tags, ",") {
		if strings.TrimSpace(t) == tag {
			return true
		}
	}
	return false
}

// ThreadTitle extracts the title from a thread (first line of first comment).
func ThreadTitle(t *Thread) string {
	if len(t.Comments) == 0 || len(t.Comments[0].Bodies) == 0 {
		return ""
	}
	content := strings.TrimSpace(t.Comments[0].Bodies[0].Content)
	if idx := strings.IndexByte(content, '\n'); idx != -1 {
		content = content[:idx]
	}
	if len(content) > 60 {
		content = content[:57] + "..."
	}
	return content
}

// NextNumber returns the next available thread number for the directory.
// Scans all existing threads and returns max(number) + 1, or 1 if empty.
// Uses streaming XML parsing to read only the root element's attributes,
// avoiding a full unmarshal of each file.
func NextNumber(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	maxNum := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".xml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		num, err := readThreadNumber(path)
		if err != nil {
			return 0, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		if num > maxNum {
			maxNum = num
		}
	}
	return maxNum + 1, nil
}

// readThreadNumber extracts just the number attribute from a thread file
// using streaming XML parsing. Stops after reading the root element's attributes.
func readThreadNumber(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	dec := xml.NewDecoder(f)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return 0, nil
		}
		if err != nil {
			return 0, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "nota-thread" {
			return 0, nil
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "number" {
				return strconv.Atoi(attr.Value)
			}
		}
		return 0, nil
	}
}

// AllThreads returns an iterator over all threads in dir.
// Errors reading individual thread files are yielded to the caller.
func AllThreads(dir string) iter.Seq2[ThreadInfo, error] {
	return func(yield func(ThreadInfo, error) bool) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return
			}
			yield(ThreadInfo{}, fmt.Errorf("reading directory %s: %w", dir, err))
			return
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".xml") {
				continue
			}

			path := filepath.Join(dir, entry.Name())
			t, err := ReadThread(path)
			if err != nil {
				if !yield(ThreadInfo{}, fmt.Errorf("reading %s: %w", entry.Name(), err)) {
					return
				}
				continue
			}

			if !yield(ThreadInfo{Thread: t, Path: path}, nil) {
				return
			}
		}
	}
}

// referencesThread returns true if t references the given thread ID.
func referencesThread(t *Thread, id string) bool {
	for _, ref := range t.Refs {
		if ref.Thread == id {
			return true
		}
	}
	return false
}

// dependsOnThread returns true if t depends on the given thread ID.
func dependsOnThread(t *Thread, id string) bool {
	for dep := range strings.SplitSeq(t.DependsOn, ",") {
		if strings.TrimSpace(dep) == id {
			return true
		}
	}
	return false
}
