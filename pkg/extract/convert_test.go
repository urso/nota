package extract

import (
	"bytes"
	"strings"
	"testing"

	"github.com/urso/nota/pkg/grouper"
	"github.com/urso/nota/pkg/parser"
)

func TestGroupToThread(t *testing.T) {
	fileContents := map[string][]byte{
		"auth.go": []byte("package auth\n\n// review(auth): check token\nfunc validate() {}\n"),
	}
	fileLines := splitFileLines(fileContents)

	g := grouper.Group{
		Name: "auth",
		Entries: []grouper.Entry{
			{
				Tag:     parser.Tag("review"),
				File:    "auth.go",
				Line:    3,
				Comment: "check token",
			},
		},
	}

	th := groupToThread(g, "abc123", fileLines)

	if th.Status != "open" {
		t.Errorf("expected status 'open', got %q", th.Status)
	}
	if th.Goal != "review" {
		t.Errorf("expected goal 'review', got %q", th.Goal)
	}
	if th.Group != "auth" {
		t.Errorf("expected group 'auth', got %q", th.Group)
	}
	if !strings.HasPrefix(th.ID, "l:") {
		t.Errorf("expected ID to start with 'l:', got %q", th.ID)
	}

	if th.Anchor == nil {
		t.Fatal("expected anchor to be set")
	}
	if th.Anchor.File != "auth.go" {
		t.Errorf("expected anchor file 'auth.go', got %q", th.Anchor.File)
	}
	if th.Anchor.Line != 3 {
		t.Errorf("expected anchor line 3, got %d", th.Anchor.Line)
	}
	if th.Anchor.Commit != "abc123" {
		t.Errorf("expected anchor commit 'abc123', got %q", th.Anchor.Commit)
	}
	if th.Anchor.ContentHash == "" {
		t.Error("expected content hash to be set")
	}

	if len(th.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(th.Comments))
	}
	if th.Comments[0].Bodies[0].Content != "check token" {
		t.Errorf("expected comment content 'check token', got %q", th.Comments[0].Bodies[0].Content)
	}
}

func TestGroupToThreadMultipleEntries(t *testing.T) {
	fileContents := map[string][]byte{
		"auth.go": []byte("line1\nline2\nline3\nline4\n"),
	}
	fileLines := splitFileLines(fileContents)

	g := grouper.Group{
		Name: "auth",
		Entries: []grouper.Entry{
			{Tag: parser.Tag("review"), File: "auth.go", Line: 1, Comment: "first"},
			{Tag: parser.Tag("discuss"), File: "auth.go", Line: 3, Comment: "second"},
		},
	}

	th := groupToThread(g, "abc123", fileLines)

	// Goal comes from first entry.
	if th.Goal != "review" {
		t.Errorf("expected goal 'review', got %q", th.Goal)
	}

	// Should have 2 comments.
	if len(th.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(th.Comments))
	}

	// Thread anchor from first entry.
	if th.Anchor.Line != 1 {
		t.Errorf("expected anchor line 1, got %d", th.Anchor.Line)
	}
}

func TestGroupToThreadWithReferences(t *testing.T) {
	g := grouper.Group{
		Name: "auth",
		Entries: []grouper.Entry{
			{Tag: parser.Tag("review"), File: "auth.go", Line: 1, Comment: "main"},
		},
		References: []grouper.Reference{
			{File: "other.go", Line: 10},
		},
	}

	th := groupToThread(g, "", nil)

	if len(th.Refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(th.Refs))
	}
	if th.Refs[0].File != "other.go:10" {
		t.Errorf("expected ref file 'other.go:10', got %q", th.Refs[0].File)
	}
}

func TestComputeContentHash(t *testing.T) {
	content := []byte("line1\nline2\nline3\n")
	lines := bytes.Split(content, []byte("\n"))

	h1 := computeContentHash(lines, 1)
	h2 := computeContentHash(lines, 2)

	if h1 == "" {
		t.Error("expected hash for line 1")
	}
	if h2 == "" {
		t.Error("expected hash for line 2")
	}
	if h1 == h2 {
		t.Error("different lines should produce different hashes")
	}

	// Verify hash format: 8 hex characters (first 4 bytes of SHA256).
	if len(h1) != 8 {
		t.Errorf("expected hash length 8, got %d: %q", len(h1), h1)
	}
	for _, c := range h1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("hash contains non-hex character: %q", h1)
			break
		}
	}

	// Verify actual hash value for known input.
	// "line1" -> SHA256 first 4 bytes -> hex
	if h1 != "815750a4" {
		t.Errorf("expected hash '815750a4' for 'line1', got %q", h1)
	}

	// Out of bounds.
	if computeContentHash(lines, 0) != "" {
		t.Error("line 0 should return empty hash")
	}
	if computeContentHash(lines, 100) != "" {
		t.Error("out of bounds line should return empty hash")
	}
	if computeContentHash(nil, 1) != "" {
		t.Error("nil content should return empty hash")
	}
}
