package service

import (
	"strings"

	"github.com/urso/nota/pkg/thread"
)

// ThreadViewJSON is the JSON encoding of a ThreadView.
type ThreadViewJSON struct {
	ID       string   `json:"id"`
	Number   int      `json:"number"`
	Status   string   `json:"status"`
	Goal     string   `json:"goal,omitempty"`
	Group    string   `json:"group,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	Title    string   `json:"title"`
	Comments []thread.Comment `json:"comments"`

	Anchor         *AnchorJSON         `json:"anchor,omitempty"`
	ResolvedAnchor *ResolvedAnchorJSON `json:"resolvedAnchor,omitempty"`
}

// AnchorJSON is the JSON encoding of a thread's stored anchor.
type AnchorJSON struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Commit      string `json:"commit,omitempty"`
	ContentHash string `json:"contentHash,omitempty"`
	Outdated    bool   `json:"outdated,omitempty"`
}

// ResolvedAnchorJSON is the JSON encoding of a resolved working-tree anchor.
type ResolvedAnchorJSON struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Outdated bool   `json:"outdated,omitempty"`
}

// ToJSON converts a ThreadView to its JSON representation.
func (v *ThreadView) ToJSON() ThreadViewJSON {
	j := ThreadViewJSON{
		ID:       v.Thread.ID,
		Number:   v.Thread.Number,
		Status:   v.Thread.Status,
		Goal:     v.Thread.Goal,
		Group:    v.Thread.Group,
		Tags:     splitTags(v.Thread.Tags),
		Title:    v.Title,
		Comments: v.Thread.Comments,
	}

	if anchor := v.Thread.CurrentAnchor(); anchor != nil {
		j.Anchor = &AnchorJSON{
			File:        anchor.File,
			Line:        anchor.Line,
			Commit:      anchor.Commit,
			ContentHash: anchor.ContentHash,
			Outdated:    anchor.Outdated,
		}
	}

	if v.ResolvedAnchor != nil {
		j.ResolvedAnchor = &ResolvedAnchorJSON{
			File:     v.ResolvedAnchor.File,
			Line:     v.ResolvedAnchor.Line,
			Outdated: v.ResolvedAnchor.Outdated,
		}
	}

	return j
}

// splitTags splits a comma-separated tags string into a slice.
func splitTags(tags string) []string {
	if tags == "" {
		return nil
	}
	var result []string
	for part := range strings.SplitSeq(tags, ",") {
		if t := strings.TrimSpace(part); t != "" {
			result = append(result, t)
		}
	}
	return result
}
