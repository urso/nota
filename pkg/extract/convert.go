package extract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/urso/nota/pkg/grouper"
	"github.com/urso/nota/pkg/thread"
)

// splitFileLines pre-splits file contents into lines for efficient per-line lookups.
func splitFileLines(fileContents map[string][]byte) map[string][][]byte {
	result := make(map[string][][]byte, len(fileContents))
	for path, content := range fileContents {
		result[path] = bytes.Split(content, []byte("\n"))
	}
	return result
}

// groupToThread converts a grouper.Group to a Thread.
// commit is the current HEAD SHA to store in anchors.
// fileLines provides pre-split file lines for computing content hashes.
// number is the thread number; 0 leaves the thread unnumbered, which is what
// named groups use since they are addressed by their stable filename instead.
//
// Per the design: each @tag(group) entry becomes a separate <nota-comment>
// with its own <nota-anchor>. The thread-level anchor is set from the first entry.
// The source tag (e.g. @review, @impl) becomes the thread's goal attribute.
func groupToThread(g grouper.Group, commit string, fileLines map[string][][]byte, number int) *thread.Thread {
	t := &thread.Thread{
		ID:     thread.NewLocalID(),
		Number: number,
		Status: "open",
		Group:  g.Name,
	}

	now := time.Now().UTC().Format(time.RFC3339)

	for i, e := range g.Entries {
		if t.Goal == "" {
			t.Goal = string(e.Tag)
		}

		anchor := &thread.Anchor{
			File:        e.File,
			Line:        e.Line,
			Commit:      commit,
			ContentHash: computeContentHash(fileLines[e.File], e.Line),
		}

		content := e.Comment
		if content == "" {
			content = fmt.Sprintf("See %s:%d", e.File, e.Line)
		}

		c := thread.Comment{
			ID:     thread.NewLocalID(),
			Author: "user",
			Anchor: anchor,
			Bodies: []thread.Body{{
				Time:    now,
				Content: content,
			}},
		}

		if i == 0 {
			t.AppendAnchor(*anchor)
		}

		t.Comments = append(t.Comments, c)
	}

	for _, r := range g.References {
		t.Refs = append(t.Refs, thread.RefFile(fmt.Sprintf("%s:%d", r.File, r.Line)))
	}

	return t
}

// computeContentHash returns SHA256 hash of the line content, first 8 hex chars.
// line is 1-indexed. lines is pre-split file content.
func computeContentHash(lines [][]byte, line int) string {
	if lines == nil || line < 1 || line > len(lines) {
		return ""
	}

	lineContent := bytes.TrimSpace(lines[line-1])
	hash := sha256.Sum256(lineContent)
	return hex.EncodeToString(hash[:4])
}
