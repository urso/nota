package deleter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ByteRange represents a range of bytes to delete.
type ByteRange struct {
	Start int64 // Inclusive
	End   int64 // Exclusive
}

// DeleteComments removes byte ranges from fileContent.
// Ranges must not overlap. Returns modified content.
func DeleteComments(fileContent []byte, ranges []ByteRange) ([]byte, error) {
	if len(ranges) == 0 {
		return fileContent, nil
	}

	// Sort ascending by Start for single-pass construction.
	sorted := make([]ByteRange, len(ranges))
	copy(sorted, ranges)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Start < sorted[j].Start
	})

	// F4: Validate all ranges against original content length before any mutations.
	origLen := int64(len(fileContent))
	for _, r := range sorted {
		if r.Start < 0 || r.End > origLen || r.Start > r.End {
			return nil, fmt.Errorf("invalid byte range [%d,%d) for content of length %d",
				r.Start, r.End, origLen)
		}
	}

	// Validate no overlaps (ascending order: ranges[i].End should be <= ranges[i+1].Start).
	for i := 0; i < len(sorted)-1; i++ {
		if sorted[i].End > sorted[i+1].Start {
			return nil, fmt.Errorf("overlapping byte ranges: [%d,%d) and [%d,%d)",
				sorted[i].Start, sorted[i].End, sorted[i+1].Start, sorted[i+1].End)
		}
	}

	// Calculate result size and build in a single pass.
	var totalDeleted int64
	for _, r := range sorted {
		totalDeleted += r.End - r.Start
	}
	result := make([]byte, 0, int64(len(fileContent))-totalDeleted)

	var pos int64
	for _, r := range sorted {
		result = append(result, fileContent[pos:r.Start]...)
		pos = r.End
	}
	result = append(result, fileContent[pos:]...)

	return result, nil
}

// WriteAtomic writes content to filePath atomically
// (temp file in same directory + rename).
func WriteAtomic(filePath string, content []byte, perm os.FileMode) error {
	dir := filepath.Dir(filePath)

	tmp, err := os.CreateTemp(dir, ".nota-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("setting permissions: %w", err)
	}

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("writing content: %w", err)
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpName, filePath); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}
