package thread

import (
	"fmt"
	"iter"
	"os"
	"path/filepath"
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

// FindThread finds a thread by ID in the given directory.
// Returns nil if not found.
func FindThread(dir, id string) (*ThreadInfo, error) {
	// Extract the ID suffix (strip "l:" prefix if present)
	suffix := id
	if strings.HasPrefix(id, "l:") {
		suffix = id[2:]
	}

	// Glob for files ending with the ID suffix
	pattern := filepath.Join(dir, "*"+suffix+".xml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob pattern %s: %w", pattern, err)
	}

	// Parse matching files and return the first exact ID match
	for _, path := range matches {
		t, err := ReadThread(path)
		if err != nil {
			continue
		}
		if t.ID == id {
			return &ThreadInfo{Thread: t, Path: path}, nil
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
