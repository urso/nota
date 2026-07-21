package thread

import (
	"fmt"
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
}

// ThreadInfo holds a thread and its source file path.
type ThreadInfo struct {
	Thread *Thread
	Path   string
}

// ListThreads scans dir for .xml files and returns threads matching the filter.
// Malformed XML files are logged to stderr and skipped.
func ListThreads(dir string, filter ThreadFilter) ([]ThreadInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	var results []ThreadInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".xml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		t, err := ReadThread(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", entry.Name(), err)
			continue
		}

		if !matchesFilter(t, filter) {
			continue
		}

		results = append(results, ThreadInfo{Thread: t, Path: path})
	}

	return results, nil
}

// FindThread finds a thread by ID in the given directory.
// Returns nil if not found.
func FindThread(dir, id string) (*ThreadInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".xml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
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
