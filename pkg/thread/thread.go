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

// Thread represents a conversation thread with optional code anchor and sync state.
type Thread struct {
	XMLName   xml.Name    `xml:"nota-thread" json:"-"`
	ID        string      `xml:"id,attr,omitempty" json:"id,omitempty"`
	Status    string      `xml:"status,attr" json:"status"`
	Goal      string      `xml:"goal,attr,omitempty" json:"goal,omitempty"`
	Group     string      `xml:"group,attr,omitempty" json:"group,omitempty"`
	Tags      string      `xml:"tags,attr,omitempty" json:"tags,omitempty"`
	DependsOn string      `xml:"depends-on,attr,omitempty" json:"dependsOn,omitempty"`
	Anchor    *Anchor     `xml:"nota-anchor,omitempty" json:"anchor,omitempty"`
	Sync      *SyncConfig `xml:"nota-sync,omitempty" json:"sync,omitempty"`
	Parent    *Parent     `xml:"nota-parent,omitempty" json:"parent,omitempty"`
	Refs      []Ref       `xml:"nota-ref,omitempty" json:"refs,omitempty"`
	Comments  []Comment   `xml:"nota-comment" json:"comments"`
}

// Parent references a parent thread.
type Parent struct {
	Ref string `xml:"ref,attr" json:"ref"`
}

// Anchor represents a code location reference.
type Anchor struct {
	File           string `xml:"file,attr" json:"file"`
	Line           int    `xml:"line,attr" json:"line"`
	Commit         string `xml:"commit,attr,omitempty" json:"commit,omitempty"`
	ContentHash    string `xml:"content-hash,attr,omitempty" json:"contentHash,omitempty"`
	TracedToCommit string `xml:"traced-to-commit,attr,omitempty" json:"tracedToCommit,omitempty"`
	Outdated       bool   `xml:"outdated,attr,omitempty" json:"outdated,omitempty"`
}

// SyncConfig holds external sync configuration for a thread.
type SyncConfig struct {
	Provider string `xml:"provider,attr" json:"provider"`
	PR       string `xml:"pr,attr,omitempty" json:"pr,omitempty"`
	ThreadID string `xml:"thread-id,attr,omitempty" json:"threadId,omitempty"`
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
	ID         string   `xml:"id,attr" json:"id"`
	Author     string   `xml:"author,attr" json:"author"`
	Visibility string   `xml:"visibility,attr,omitempty" json:"visibility,omitempty"`
	SyncStatus string   `xml:"sync-status,attr,omitempty" json:"syncStatus,omitempty"`
	Anchor     *Anchor  `xml:"nota-anchor,omitempty" json:"anchor,omitempty"`
	ReplyTo    *ReplyTo `xml:"nota-reply-to,omitempty" json:"replyTo,omitempty"`
	Bodies     []Body   `xml:"nota-body" json:"bodies"`
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
// Uses crypto/rand with fallback to the global PRNG if crypto/rand fails.
func NewLocalID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = byte(mrand.Uint32())
		}
	}
	return "l:" + hex.EncodeToString(b)
}
