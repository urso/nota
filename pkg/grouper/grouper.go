package grouper

import (
	"fmt"
	"os"
	"sort"

	"github.com/urso/aireview/pkg/parser"
)

// Entry is a review comment with its surrounding context.
type Entry struct {
	Tag     parser.Tag
	File    string
	Line    int
	Comment string
	Context ContextLines
}

// Reference is a see:/also: cross-reference with context only (no comment text).
type Reference struct {
	File    string
	Line    int
	Context ContextLines
}

// Group is a named collection of related review comments.
type Group struct {
	Name       string
	Entries    []Entry
	References []Reference
}

// GroupComments takes review comments and groups them by name.
// see/also tags become References.
// Unnamed comments become standalone groups (one entry, no name).
func GroupComments(comments []parser.ReviewComment, files map[string][]byte, contextLines int) []Group {
	namedGroups := make(map[string]*Group)
	var namedOrder []string
	var unnamed []Group

	// Collect references for second pass.
	type pendingRef struct {
		name string
		ref  Reference
		c    parser.ReviewComment
	}
	var refs []pendingRef

	// First pass: collect named groups and unnamed entries.
	for _, c := range comments {
		ctx := extractContext(files[c.File], c.Line, c.EndLine, contextLines)

		switch c.Tag {
		case parser.TagSee, parser.TagAlso:
			refs = append(refs, pendingRef{
				name: c.Name,
				ref: Reference{
					File:    c.File,
					Line:    c.Line,
					Context: ctx,
				},
				c: c,
			})

		case parser.TagReview, parser.TagDiscuss, parser.TagExplain:
			entry := Entry{
				Tag:     c.Tag,
				File:    c.File,
				Line:    c.Line,
				Comment: c.Message,
				Context: ctx,
			}

			if c.Name == "" {
				unnamed = append(unnamed, Group{
					Name:    "",
					Entries: []Entry{entry},
				})
			} else {
				g, ok := namedGroups[c.Name]
				if !ok {
					g = &Group{Name: c.Name}
					namedGroups[c.Name] = g
					namedOrder = append(namedOrder, c.Name)
				}
				g.Entries = append(g.Entries, entry)
			}
		}
	}

	// Second pass: attach references to named groups.
	for _, pr := range refs {
		g, ok := namedGroups[pr.name]
		if !ok {
			fmt.Fprintf(os.Stderr, "warning: %s references unknown group %q at %s:%d\n",
				pr.c.Tag, pr.name, pr.c.File, pr.c.Line)
			continue
		}
		g.References = append(g.References, pr.ref)
	}

	// Sort named groups alphabetically.
	sort.Strings(namedOrder)

	var result []Group
	for _, name := range namedOrder {
		result = append(result, *namedGroups[name])
	}
	result = append(result, unnamed...)

	return result
}
