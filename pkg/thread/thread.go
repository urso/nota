// Package thread provides XML-based thread types for nota review conversations.
package thread

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	mrand "math/rand/v2"
	"slices"
	"time"
)

// GoalValues lists all valid goal values.
var GoalValues = []string{"review", "discuss", "impl", "explain", "refactor", "test", "doc", "propose", "critique"}

// ValidGoal returns true if the goal is valid.
func ValidGoal(goal string) bool {
	return slices.Contains(GoalValues, goal)
}

// Thread represents a conversation thread with optional code anchors and sync state.
type Thread struct {
	XMLName xml.Name `xml:"nota-thread" json:"-"`
	ID      string   `xml:"id,attr,omitempty" json:"id,omitempty"`
	Number  int      `xml:"number,attr,omitempty" json:"number,omitempty"`
	Status  string   `xml:"status,attr" json:"status"`
	Goal    string   `xml:"goal,attr,omitempty" json:"goal,omitempty"`
	Group   string   `xml:"group,attr,omitempty" json:"group,omitempty"`
	Tags    string   `xml:"tags,attr,omitempty" json:"tags,omitempty"`

	DependsOn string   `xml:"depends-on,attr,omitempty" json:"dependsOn,omitempty"`
	Anchors   []Anchor `xml:"nota-anchor,omitempty" json:"anchors,omitempty"`

	// FileAnchors are file-level review positions with no line number, kept
	// separate from Anchors so they can never reach the line tracer.
	FileAnchors []FileAnchor `xml:"nota-anchor-file,omitempty" json:"fileAnchors,omitempty"`
	Sync        *SyncConfig  `xml:"nota-sync,omitempty" json:"sync,omitempty"`
	Parent      *Parent      `xml:"nota-parent,omitempty" json:"parent,omitempty"`
	Refs        []Ref        `xml:"nota-ref,omitempty" json:"refs,omitempty"`
	Comments    []Comment    `xml:"nota-comment" json:"comments"`
}

// CurrentAnchor returns the latest line anchor, or nil if no line anchors exist.
// File anchors are deliberately excluded: they carry no line and must never be traced.
func (t *Thread) CurrentAnchor() *Anchor {
	if len(t.Anchors) == 0 {
		return nil
	}
	return &t.Anchors[len(t.Anchors)-1]
}

// Location holds the display-relevant fields from either anchor type.
type Location struct {
	File     string
	Line     int // 0 for file-level anchors
	Commit   string
	Outdated bool // always false for file anchors
}

// CurrentLocation returns the thread's current location for display, preferring
// the latest line anchor and falling back to the latest file anchor.
// Returns nil if the thread has no anchors of either kind.
func (t *Thread) CurrentLocation() *Location {
	if a := t.CurrentAnchor(); a != nil {
		return &Location{File: a.File, Line: a.Line, Commit: a.Commit, Outdated: a.Outdated}
	}
	if len(t.FileAnchors) == 0 {
		return nil
	}
	fa := &t.FileAnchors[len(t.FileAnchors)-1]
	return &Location{File: fa.File, Commit: fa.Commit}
}

// AppendAnchor adds a new anchor to the thread's anchor history.
func (t *Thread) AppendAnchor(a Anchor) {
	t.Anchors = append(t.Anchors, a)
}

// AppendFileAnchor adds a new file-level anchor to the thread.
func (t *Thread) AppendFileAnchor(a FileAnchor) {
	t.FileAnchors = append(t.FileAnchors, a)
}

// Parent references a parent thread.
type Parent struct {
	Ref string `xml:"ref,attr" json:"ref"`
}

// Anchor represents a code location reference at a specific commit.
// Anchors are append-only; each trace adds a new anchor with TracedFrom linking to the source.
type Anchor struct {
	File        string `xml:"file,attr" json:"file"`
	Line        int    `xml:"line,attr" json:"line"`
	Commit      string `xml:"commit,attr,omitempty" json:"commit,omitempty"`
	ContentHash string `xml:"content-hash,attr,omitempty" json:"contentHash,omitempty"`
	TracedFrom  string `xml:"traced-from,attr,omitempty" json:"tracedFrom,omitempty"`
	Outdated    bool   `xml:"outdated,attr,omitempty" json:"outdated,omitempty"`
}

// FileAnchor represents a file-level review position with no line number.
// Unlike Anchor, file anchors are never traced and have no line or outdated state.
type FileAnchor struct {
	File        string `xml:"file,attr" json:"file"`
	Commit      string `xml:"commit,attr,omitempty" json:"commit,omitempty"`
	ContentHash string `xml:"content-hash,attr,omitempty" json:"contentHash,omitempty"`
}

// Sync kinds distinguish an inline review thread from the PR conversation container.
const (
	// SyncKindReviewThread is an inline review thread, identified by ThreadID.
	SyncKindReviewThread = "review-thread"
	// SyncKindPR is the PR conversation container, which has no remote thread ID.
	SyncKindPR = "pr"
)

// SyncKindValues lists all valid sync kinds.
var SyncKindValues = []string{SyncKindReviewThread, SyncKindPR}

// SyncConfig holds external sync configuration for a thread.
type SyncConfig struct {
	Provider string `xml:"provider,attr" json:"provider"`
	PR       string `xml:"pr,attr,omitempty" json:"pr,omitempty"`
	ThreadID string `xml:"thread-id,attr,omitempty" json:"threadId,omitempty"`

	// Kind is "review-thread" or "pr". Empty means "review-thread".
	Kind string `xml:"kind,attr,omitempty" json:"kind,omitempty"`
	// PRID is the pull request's own node ID, reference data for push.
	PRID string `xml:"pr-id,attr,omitempty" json:"prId,omitempty"`
	// Repo is "owner/name", written only when it differs from the repo the
	// pull resolved against. Absent means "the current repo".
	Repo string `xml:"repo,attr,omitempty" json:"repo,omitempty"`
}

// EffectiveKind returns the sync kind, defaulting to "review-thread" when unset.
func (s *SyncConfig) EffectiveKind() string {
	if s.Kind == "" {
		return SyncKindReviewThread
	}
	return s.Kind
}

// Ref is a typed reference to another thread, file, or external link.
// Exactly one of Thread, File, or Link should be set.
type Ref struct {
	Thread string `xml:"thread,attr,omitempty" json:"thread,omitempty"`
	File   string `xml:"file,attr,omitempty" json:"file,omitempty"`
	Link   string `xml:"link,attr,omitempty" json:"link,omitempty"`
}

// RefThread creates a reference to another thread.
func RefThread(id string) Ref {
	return Ref{Thread: id}
}

// RefFile creates a reference to a project file.
func RefFile(path string) Ref {
	return Ref{File: path}
}

// RefLink creates a reference to an external URL.
func RefLink(url string) Ref {
	return Ref{Link: url}
}

// Comment is a single comment within a thread.
type Comment struct {
	ID         string `xml:"id,attr" json:"id"`
	Author     string `xml:"author,attr" json:"author"`
	Visibility string `xml:"visibility,attr,omitempty" json:"visibility,omitempty"`
	SyncStatus string `xml:"sync-status,attr,omitempty" json:"syncStatus,omitempty"`

	// ExternalID is the remote numeric ID, stored as a string because GitHub
	// returns fullDatabaseId as a JSON string and real IDs exceed int32.
	ExternalID string `xml:"external-id,attr,omitempty" json:"externalId,omitempty"`
	// UpdatedAt is the remote update timestamp observed at pull time; a change
	// between stored and freshly fetched values signals an edit.
	UpdatedAt string `xml:"updated-at,attr,omitempty" json:"updatedAt,omitempty"`

	Anchor  *Anchor  `xml:"nota-anchor,omitempty" json:"anchor,omitempty"`
	ReplyTo *ReplyTo `xml:"nota-reply-to,omitempty" json:"replyTo,omitempty"`
	Bodies  []Body   `xml:"nota-body" json:"bodies"`
}

// ReplyTo references another comment this comment is replying to.
type ReplyTo struct {
	Ref string `xml:"ref,attr" json:"ref"`
}

// Body contains comment content with timestamp. Content is wrapped in CDATA.
type Body struct {
	Time    string `xml:"time,attr" json:"time"`
	Content string `xml:"-" json:"content"`
}

// MarshalXML implements custom marshaling with proper XML escaping.
func (b Body) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	type bodyEscaped struct {
		XMLName xml.Name `xml:"nota-body"`
		Time    string   `xml:"time,attr"`
		Content string   `xml:",chardata"`
	}
	return e.Encode(bodyEscaped{
		Time:    b.Time,
		Content: b.Content,
	})
}

// UnmarshalXML implements custom unmarshaling to extract Content from CDATA or plain text.
func (b *Body) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "time" {
			b.Time = attr.Value
		}
	}

	var content string
	if err := d.DecodeElement(&content, &start); err != nil {
		return err
	}
	b.Content = content
	return nil
}

// NewThread creates a new thread with a generated local ID.
func NewThread(status, goal string) *Thread {
	return &Thread{
		ID:     NewLocalID(),
		Status: status,
		Goal:   goal,
	}
}

// NewComment creates a new comment with a generated local ID.
func NewComment(author, content string) Comment {
	return Comment{
		ID:     NewLocalID(),
		Author: author,
		Bodies: []Body{{
			Time:    time.Now().UTC().Format(time.RFC3339),
			Content: content,
		}},
	}
}

// NewLocalID generates a local ID with "l:" prefix and random hex suffix.
func NewLocalID() string {
	return "l:" + randomHex()
}

// NewGitHubID generates a thread ID with "gh:" prefix and random hex suffix.
// The prefix marks origin only; the remote identity lives in <nota-sync>.
func NewGitHubID() string {
	return "gh:" + randomHex()
}

// randomHex returns 16 hex characters from crypto/rand, falling back to the
// global PRNG if crypto/rand fails.
func randomHex() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = byte(mrand.Uint32())
		}
	}
	return hex.EncodeToString(b)
}
