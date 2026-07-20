// Package thread provides XML-based thread types for nota review conversations.
package thread

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"time"
)

// Thread represents a conversation thread with optional code anchor and sync state.
type Thread struct {
	XMLName   xml.Name    `xml:"nota-thread"`
	ID        string      `xml:"id,attr,omitempty"`
	Status    string      `xml:"status,attr"`
	Goal      string      `xml:"goal,attr,omitempty"`
	Group     string      `xml:"group,attr,omitempty"`
	Tags      string      `xml:"tags,attr,omitempty"`
	DependsOn string      `xml:"depends-on,attr,omitempty"`
	Anchor    *Anchor     `xml:"nota-anchor,omitempty"`
	Sync      *SyncConfig `xml:"nota-sync,omitempty"`
	Parent    *Parent     `xml:"nota-parent,omitempty"`
	Refs      []Ref       `xml:"nota-ref,omitempty"`
	Comments  []Comment   `xml:"nota-comment"`
}

// Parent references a parent thread.
type Parent struct {
	Ref string `xml:"ref,attr"`
}

// Anchor represents a code location reference.
type Anchor struct {
	File           string `xml:"file,attr"`
	Line           int    `xml:"line,attr"`
	Commit         string `xml:"commit,attr,omitempty"`
	ContentHash    string `xml:"content-hash,attr,omitempty"`
	TracedToCommit string `xml:"traced-to-commit,attr,omitempty"`
	Outdated       bool   `xml:"outdated,attr,omitempty"`
}

// SyncConfig holds external sync configuration for a thread.
type SyncConfig struct {
	Provider string `xml:"provider,attr"`
	PR       string `xml:"pr,attr,omitempty"`
	ThreadID string `xml:"thread-id,attr,omitempty"`
}

// Ref is a typed reference to another thread, file, or external link.
// Exactly one of Thread, File, or Link should be set.
type Ref struct {
	Thread string `xml:"thread,attr,omitempty"`
	File   string `xml:"file,attr,omitempty"`
	Link   string `xml:"link,attr,omitempty"`
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
	ID         string `xml:"id,attr"`
	Author     string `xml:"author,attr"`
	Visibility string `xml:"visibility,attr,omitempty"`
	SyncStatus string `xml:"sync-status,attr,omitempty"`
	ReplyTo    string `xml:"reply-to,attr,omitempty"`
	Bodies     []Body `xml:"nota-body"`
}

// Body contains comment content with timestamp. Content is wrapped in CDATA.
type Body struct {
	Time    string `xml:"time,attr"`
	Content string `xml:"-"`
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
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random ID: %v", err))
	}
	return "l:" + hex.EncodeToString(b)
}
