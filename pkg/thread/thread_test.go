package thread

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	th := &Thread{
		ID:        "l:test123",
		Status:    "open",
		Goal:      "review",
		Group:     "pr-487",
		Tags:      "auth,security",
		DependsOn: "",
		Anchors: []Anchor{
			{
				File:        "handlers/auth.go",
				Line:        42,
				Commit:      "abc123",
				ContentHash: "a1b2c3d4",
			},
		},
		FileAnchors: []FileAnchor{
			{
				File:   "handlers/auth.go",
				Commit: "def456",
			},
		},
		Sync: &SyncConfig{
			Provider: "github",
			PR:       "487",
			ThreadID: "PRRT_kwDOAbCdEf",
			Kind:     SyncKindReviewThread,
			PRID:     "PR_kwDOAbCdEf",
			Repo:     "owner/name",
		},
		Parent: &Parent{Ref: "pr-487-review"},
		Refs: []Ref{
			{Thread: "t:auth-design"},
			{File: "docs/auth-spec.md"},
			{Link: "https://github.com/org/repo/issues/123"},
		},
		Comments: []Comment{
			{
				ID:     "l:a1b2",
				Author: "user",
				Bodies: []Body{
					{Time: "2026-07-16T10:30:00Z", Content: "Check token expiry before proceeding."},
				},
			},
			{
				ID:         "gh:PRRC_kwDOAbCdEf",
				Author:     "github:alice",
				SyncStatus: "pulled",
				ExternalID: "3334261373",
				UpdatedAt:  "2026-07-16T10:36:00Z",
				Bodies: []Body{
					{Time: "2026-07-16T10:35:00Z", Content: "What about refresh tokens?"},
				},
			},
		},
	}

	// Marshal
	data1, err := MarshalThread(th)
	if err != nil {
		t.Fatalf("first marshal failed: %v", err)
	}

	// Unmarshal
	th2, err := UnmarshalThread(data1, "test.xml")
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Marshal again
	data2, err := MarshalThread(th2)
	if err != nil {
		t.Fatalf("second marshal failed: %v", err)
	}

	// Compare
	if string(data1) != string(data2) {
		t.Errorf("round-trip mismatch:\n--- first ---\n%s\n--- second ---\n%s", data1, data2)
	}
}

func TestRoundTripPerCommentAnchors(t *testing.T) {
	th := &Thread{
		ID:     "l:test456",
		Status: "open",
		Goal:   "review",
		Anchors: []Anchor{
			{
				File:   "main.go",
				Line:   10,
				Commit: "abc123",
			},
		},
		Comments: []Comment{
			{
				ID:     "l:c1",
				Author: "user",
				Anchor: &Anchor{
					File:        "handlers/auth.go",
					Line:        42,
					Commit:      "abc123",
					ContentHash: "a1b2c3d4",
				},
				Bodies: []Body{
					{Time: "2026-07-16T10:30:00Z", Content: "First comment with anchor"},
				},
			},
			{
				ID:     "l:c2",
				Author: "user",
				Bodies: []Body{
					{Time: "2026-07-16T10:35:00Z", Content: "Second comment without anchor"},
				},
			},
			{
				ID:     "l:c3",
				Author: "user",
				Anchor: &Anchor{
					File:        "handlers/auth.go",
					Line:        78,
					Commit:      "abc123",
					ContentHash: "e5f6g7h8",
				},
				Bodies: []Body{
					{Time: "2026-07-16T10:40:00Z", Content: "Third comment with different anchor"},
				},
			},
		},
	}

	data1, err := MarshalThread(th)
	if err != nil {
		t.Fatalf("first marshal failed: %v", err)
	}

	th2, err := UnmarshalThread(data1, "test.xml")
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	data2, err := MarshalThread(th2)
	if err != nil {
		t.Fatalf("second marshal failed: %v", err)
	}

	if string(data1) != string(data2) {
		t.Errorf("round-trip mismatch:\n--- first ---\n%s\n--- second ---\n%s", data1, data2)
	}

	// Verify anchors were preserved
	if th2.CurrentAnchor() == nil || th2.CurrentAnchor().File != "main.go" {
		t.Error("thread-level anchor not preserved")
	}
	if th2.Comments[0].Anchor == nil || th2.Comments[0].Anchor.Line != 42 {
		t.Error("first comment anchor not preserved")
	}
	if th2.Comments[1].Anchor != nil {
		t.Error("second comment should have no anchor")
	}
	if th2.Comments[2].Anchor == nil || th2.Comments[2].Anchor.Line != 78 {
		t.Error("third comment anchor not preserved")
	}
}

func TestRoundTripReplyTo(t *testing.T) {
	th := &Thread{
		ID:     "l:replytest",
		Status: "open",
		Comments: []Comment{
			{
				ID:     "l:c1",
				Author: "alice",
				Bodies: []Body{{Time: "2026-07-16T10:00:00Z", Content: "Original"}},
			},
			{
				ID:      "l:c2",
				Author:  "bob",
				ReplyTo: &ReplyTo{Ref: "l:c1"},
				Bodies:  []Body{{Time: "2026-07-16T10:05:00Z", Content: "Reply to alice"}},
			},
		},
	}

	data1, err := MarshalThread(th)
	if err != nil {
		t.Fatalf("first marshal failed: %v", err)
	}

	// Verify XML contains nota-reply-to element, not attribute
	xml := string(data1)
	if !strings.Contains(xml, `<nota-reply-to ref="l:c1"></nota-reply-to>`) {
		t.Errorf("expected <nota-reply-to ref=...> element in XML:\n%s", xml)
	}
	if strings.Contains(xml, `reply-to="l:c1"`) {
		t.Errorf("should not have reply-to attribute in XML:\n%s", xml)
	}

	th2, err := UnmarshalThread(data1, "test.xml")
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	data2, err := MarshalThread(th2)
	if err != nil {
		t.Fatalf("second marshal failed: %v", err)
	}

	if string(data1) != string(data2) {
		t.Errorf("round-trip mismatch:\n--- first ---\n%s\n--- second ---\n%s", data1, data2)
	}

	// Verify ReplyTo preserved
	if th2.Comments[0].ReplyTo != nil {
		t.Error("first comment should have no reply-to")
	}
	if th2.Comments[1].ReplyTo == nil || th2.Comments[1].ReplyTo.Ref != "l:c1" {
		t.Errorf("reply-to not preserved: %v", th2.Comments[1].ReplyTo)
	}
}

func TestRoundTripSpecialChars(t *testing.T) {
	content := `Check token expiry.

## Details

The code at line 78 has the same issue:

` + "```go\ntoken := refreshToken(ctx)\n```" + `

Special chars: <tag> & "quotes" and ]]> CDATA end marker`

	th := &Thread{
		Status: "open",
		Comments: []Comment{
			{
				ID:     "l:test",
				Author: "user",
				Bodies: []Body{
					{Time: "2026-07-16T10:30:00Z", Content: content},
				},
			},
		},
	}

	data, err := MarshalThread(th)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	th2, err := UnmarshalThread(data, "test.xml")
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if th2.Comments[0].Bodies[0].Content != content {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", th2.Comments[0].Bodies[0].Content, content)
	}
}

func TestMarshalEscapesXMLContent(t *testing.T) {
	th := &Thread{
		Status: "open",
		Comments: []Comment{
			{
				ID:     "l:test",
				Author: "user",
				Bodies: []Body{
					{Time: "2026-07-16T10:30:00Z", Content: "Test <content> with ]]> special chars"},
				},
			},
		},
	}

	data, err := MarshalThread(th)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	s := string(data)
	if !strings.Contains(s, "&lt;content&gt;") {
		t.Errorf("expected escaped angle brackets in output:\n%s", s)
	}
	if !strings.Contains(s, "]]&gt;") {
		t.Errorf("expected escaped CDATA end marker in output:\n%s", s)
	}
}

func TestMarshalContainsStylesheet(t *testing.T) {
	th := &Thread{
		Status: "open",
		Comments: []Comment{
			{
				ID:     "l:test",
				Author: "user",
				Bodies: []Body{
					{Time: "2026-07-16T10:30:00Z", Content: "Test"},
				},
			},
		},
	}

	data, err := MarshalThread(th)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if !strings.Contains(string(data), `<?xml-stylesheet type="text/xsl" href="nota.xslt"?>`) {
		t.Errorf("expected stylesheet PI in output:\n%s", data)
	}
}

func TestWriteReadThread(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-thread.xml")

	th := &Thread{
		ID:     "l:iotest",
		Status: "open",
		Goal:   "review",
		Comments: []Comment{
			{
				ID:     "l:c1",
				Author: "tester",
				Bodies: []Body{
					{Time: "2026-07-16T12:00:00Z", Content: "File I/O test"},
				},
			},
		},
	}

	if err := WriteThread(path, th); err != nil {
		t.Fatalf("WriteThread failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file not created: %v", err)
	}

	// Read it back
	th2, err := ReadThread(path)
	if err != nil {
		t.Fatalf("ReadThread failed: %v", err)
	}

	if th2.ID != th.ID {
		t.Errorf("ID mismatch: got %q, want %q", th2.ID, th.ID)
	}
	if th2.Status != th.Status {
		t.Errorf("Status mismatch: got %q, want %q", th2.Status, th.Status)
	}
	if th2.Goal != th.Goal {
		t.Errorf("Goal mismatch: got %q, want %q", th2.Goal, th.Goal)
	}
	if len(th2.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(th2.Comments))
	}
	if th2.Comments[0].Bodies[0].Content != "File I/O test" {
		t.Errorf("Content mismatch: got %q", th2.Comments[0].Bodies[0].Content)
	}
}

func TestValidateThread(t *testing.T) {
	tests := []struct {
		name    string
		thread  *Thread
		wantErr string
	}{
		{
			name:    "missing status",
			thread:  &Thread{Comments: []Comment{{ID: "l:1", Author: "u", Bodies: []Body{{Time: "t", Content: "c"}}}}},
			wantErr: "missing required field: status",
		},
		{
			name:    "invalid status",
			thread:  &Thread{Status: "invalid", Comments: []Comment{{ID: "l:1", Author: "u", Bodies: []Body{{Time: "t", Content: "c"}}}}},
			wantErr: "invalid status",
		},
		{
			name:    "no comments",
			thread:  &Thread{Status: "open"},
			wantErr: "must have at least one comment",
		},
		{
			name:    "comment missing id",
			thread:  &Thread{Status: "open", Comments: []Comment{{Author: "u", Bodies: []Body{{Time: "t", Content: "c"}}}}},
			wantErr: "comment 0: missing required field: id",
		},
		{
			name:    "comment missing author",
			thread:  &Thread{Status: "open", Comments: []Comment{{ID: "l:1", Bodies: []Body{{Time: "t", Content: "c"}}}}},
			wantErr: "comment 0: missing required field: author",
		},
		{
			name:    "comment no bodies",
			thread:  &Thread{Status: "open", Comments: []Comment{{ID: "l:1", Author: "u"}}},
			wantErr: "comment 0: must have at least one body",
		},
		{
			name:    "body missing time",
			thread:  &Thread{Status: "open", Comments: []Comment{{ID: "l:1", Author: "u", Bodies: []Body{{Content: "c"}}}}},
			wantErr: "body 0: missing required field: time",
		},
		{
			name:    "ref with no attributes",
			thread:  &Thread{Status: "open", Refs: []Ref{{}}, Comments: []Comment{{ID: "l:1", Author: "u", Bodies: []Body{{Time: "t", Content: "c"}}}}},
			wantErr: "ref 0: must have exactly one of thread, file, or link",
		},
		{
			name:    "ref with multiple attributes",
			thread:  &Thread{Status: "open", Refs: []Ref{{Thread: "t:1", File: "f.go"}}, Comments: []Comment{{ID: "l:1", Author: "u", Bodies: []Body{{Time: "t", Content: "c"}}}}},
			wantErr: "ref 0: must have exactly one of thread, file, or link (has 2)",
		},
		{
			name: "sync missing provider",
			thread: &Thread{
				Status:   "open",
				Sync:     &SyncConfig{PR: "487", ThreadID: "PRRT_x"},
				Comments: []Comment{{ID: "l:1", Author: "u", Bodies: []Body{{Time: "t", Content: "c"}}}},
			},
			wantErr: "sync: missing required field: provider",
		},
		{
			name: "sync invalid kind",
			thread: &Thread{
				Status:   "open",
				Sync:     &SyncConfig{Provider: "github", PR: "487", ThreadID: "PRRT_x", Kind: "issue"},
				Comments: []Comment{{ID: "l:1", Author: "u", Bodies: []Body{{Time: "t", Content: "c"}}}},
			},
			wantErr: `sync: invalid kind "issue"`,
		},
		{
			name: "sync missing thread-id defaults to review-thread and is malformed",
			thread: &Thread{
				Status:   "open",
				Sync:     &SyncConfig{Provider: "github", PR: "487"},
				Comments: []Comment{{ID: "l:1", Author: "u", Bodies: []Body{{Time: "t", Content: "c"}}}},
			},
			wantErr: "sync: missing required field: thread-id",
		},
		{
			name: "sync review-thread without thread-id",
			thread: &Thread{
				Status:   "open",
				Sync:     &SyncConfig{Provider: "github", PR: "487", Kind: SyncKindReviewThread},
				Comments: []Comment{{ID: "l:1", Author: "u", Bodies: []Body{{Time: "t", Content: "c"}}}},
			},
			wantErr: "sync: missing required field: thread-id",
		},
		{
			name: "valid sync review-thread",
			thread: &Thread{
				Status:   "open",
				Sync:     &SyncConfig{Provider: "github", PR: "487", ThreadID: "PRRT_kwDO", Kind: SyncKindReviewThread, PRID: "PR_kwDO"},
				Comments: []Comment{{ID: "l:1", Author: "u", Bodies: []Body{{Time: "t", Content: "c"}}}},
			},
			wantErr: "",
		},
		{
			name: "valid sync pr conversation has no thread-id",
			thread: &Thread{
				Status:   "open",
				Sync:     &SyncConfig{Provider: "github", PR: "487", Kind: SyncKindPR, PRID: "PR_kwDO"},
				Comments: []Comment{{ID: "l:1", Author: "u", Bodies: []Body{{Time: "t", Content: "c"}}}},
			},
			wantErr: "",
		},
		{
			name: "valid thread",
			thread: &Thread{
				Status:   "open",
				Comments: []Comment{{ID: "l:1", Author: "user", Bodies: []Body{{Time: "2026-07-16T10:00:00Z", Content: "test"}}}},
			},
			wantErr: "",
		},
		{
			name: "valid resolved",
			thread: &Thread{
				Status:   "resolved",
				Comments: []Comment{{ID: "l:1", Author: "user", Bodies: []Body{{Time: "2026-07-16T10:00:00Z", Content: "test"}}}},
			},
			wantErr: "",
		},
		{
			name: "valid wontfix",
			thread: &Thread{
				Status:   "wontfix",
				Comments: []Comment{{ID: "l:1", Author: "user", Bodies: []Body{{Time: "2026-07-16T10:00:00Z", Content: "test"}}}},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateThread(tt.thread)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
				}
			}
		})
	}
}

func TestRoundTripFileAnchors(t *testing.T) {
	th := &Thread{
		ID:     "l:fileanchor",
		Status: "open",
		Goal:   "review",
		FileAnchors: []FileAnchor{
			{File: "handlers/auth.go", Commit: "abc123"},
		},
		Comments: []Comment{
			{ID: "l:c1", Author: "github:alice", Bodies: []Body{{Time: "2026-07-16T10:00:00Z", Content: "This file needs a package doc."}}},
		},
	}

	data1, err := MarshalThread(th)
	if err != nil {
		t.Fatalf("first marshal failed: %v", err)
	}

	if !strings.Contains(string(data1), "<nota-anchor-file ") {
		t.Errorf("expected <nota-anchor-file> element, got:\n%s", data1)
	}

	th2, err := UnmarshalThread(data1, "test.xml")
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	data2, err := MarshalThread(th2)
	if err != nil {
		t.Fatalf("second marshal failed: %v", err)
	}
	if string(data1) != string(data2) {
		t.Errorf("round-trip mismatch:\n--- first ---\n%s\n--- second ---\n%s", data1, data2)
	}

	// A file anchor must never be reachable through the tracer's entry point.
	if th2.CurrentAnchor() != nil {
		t.Error("file anchor leaked into CurrentAnchor(); it would be traced as a line anchor")
	}
	if len(th2.FileAnchors) != 1 || th2.FileAnchors[0].File != "handlers/auth.go" {
		t.Errorf("file anchor not preserved: %+v", th2.FileAnchors)
	}
}

func TestCurrentLocation(t *testing.T) {
	lineAnchor := Anchor{File: "a.go", Line: 42, Commit: "abc"}
	fileAnchor := FileAnchor{File: "b.go", Commit: "def"}

	tests := []struct {
		name     string
		thread   *Thread
		wantNil  bool
		wantFile string
		wantLine int
	}{
		{name: "no anchors", thread: &Thread{}, wantNil: true},
		{
			name:     "line anchor only",
			thread:   &Thread{Anchors: []Anchor{lineAnchor}},
			wantFile: "a.go",
			wantLine: 42,
		},
		{
			name:     "file anchor only",
			thread:   &Thread{FileAnchors: []FileAnchor{fileAnchor}},
			wantFile: "b.go",
			wantLine: 0,
		},
		{
			name:     "line anchor wins over file anchor",
			thread:   &Thread{Anchors: []Anchor{lineAnchor}, FileAnchors: []FileAnchor{fileAnchor}},
			wantFile: "a.go",
			wantLine: 42,
		},
		{
			name:     "latest line anchor",
			thread:   &Thread{Anchors: []Anchor{lineAnchor, {File: "c.go", Line: 7}}},
			wantFile: "c.go",
			wantLine: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.thread.CurrentLocation()
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil location, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected a location, got nil")
			}
			if got.File != tt.wantFile {
				t.Errorf("expected file %q, got %q", tt.wantFile, got.File)
			}
			if got.Line != tt.wantLine {
				t.Errorf("expected line %d, got %d", tt.wantLine, got.Line)
			}
		})
	}
}

func TestNewGitHubID(t *testing.T) {
	id := NewGitHubID()
	if err := ValidateID(id); err != nil {
		t.Errorf("NewGitHubID() produced %q, which fails ValidateID: %v", id, err)
	}
	if !strings.HasPrefix(id, "gh:") {
		t.Errorf("expected ID to start with 'gh:', got %q", id)
	}
	if id == NewGitHubID() {
		t.Error("expected distinct IDs across calls")
	}
}

func TestNewThread(t *testing.T) {
	th := NewThread("open", "review")
	if th.Status != "open" {
		t.Errorf("expected status 'open', got %q", th.Status)
	}
	if th.Goal != "review" {
		t.Errorf("expected goal 'review', got %q", th.Goal)
	}
	// Verify ID format: "l:" prefix followed by hex characters.
	if !regexp.MustCompile(`^l:[0-9a-f]+$`).MatchString(th.ID) {
		t.Errorf("expected ID to match 'l:<hex>', got %q", th.ID)
	}
}

func TestNewComment(t *testing.T) {
	c := NewComment("testuser", "Test message")
	if c.Author != "testuser" {
		t.Errorf("expected author 'testuser', got %q", c.Author)
	}
	if !strings.HasPrefix(c.ID, "l:") {
		t.Errorf("expected ID to start with 'l:', got %q", c.ID)
	}
	if len(c.Bodies) != 1 {
		t.Fatalf("expected 1 body, got %d", len(c.Bodies))
	}
	if c.Bodies[0].Content != "Test message" {
		t.Errorf("expected content 'Test message', got %q", c.Bodies[0].Content)
	}
	if c.Bodies[0].Time == "" {
		t.Error("expected non-empty time")
	}
}

func TestRefConstructors(t *testing.T) {
	r1 := RefThread("t:abc")
	if r1.Thread != "t:abc" || r1.File != "" || r1.Link != "" {
		t.Errorf("RefThread incorrect: %+v", r1)
	}

	r2 := RefFile("path/to/file.go")
	if r2.File != "path/to/file.go" || r2.Thread != "" || r2.Link != "" {
		t.Errorf("RefFile incorrect: %+v", r2)
	}

	r3 := RefLink("https://example.com")
	if r3.Link != "https://example.com" || r3.Thread != "" || r3.File != "" {
		t.Errorf("RefLink incorrect: %+v", r3)
	}
}

func TestReadThreadNotFound(t *testing.T) {
	_, err := ReadThread("/nonexistent/path.xml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestUnmarshalInvalidXML(t *testing.T) {
	_, err := UnmarshalThread([]byte("not xml"), "test.xml")
	if err == nil {
		t.Error("expected error for invalid XML")
	}
	if !strings.Contains(err.Error(), "parsing test.xml") {
		t.Errorf("expected error to mention file path, got: %v", err)
	}
}

func TestMultipleBodies(t *testing.T) {
	th := &Thread{
		Status: "open",
		Comments: []Comment{
			{
				ID:     "gh:123",
				Author: "github:alice",
				Bodies: []Body{
					{Time: "2026-07-16T10:00:00Z", Content: "Original message"},
					{Time: "2026-07-16T10:30:00Z", Content: "Edited message with more detail"},
				},
			},
		},
	}

	data, err := MarshalThread(th)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	th2, err := UnmarshalThread(data, "test.xml")
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(th2.Comments[0].Bodies) != 2 {
		t.Fatalf("expected 2 bodies, got %d", len(th2.Comments[0].Bodies))
	}
	if th2.Comments[0].Bodies[0].Content != "Original message" {
		t.Errorf("first body content mismatch")
	}
	if th2.Comments[0].Bodies[1].Content != "Edited message with more detail" {
		t.Errorf("second body content mismatch")
	}
}
