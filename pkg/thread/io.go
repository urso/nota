package thread

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/urso/nota/pkg/deleter"
)

// WriteThread writes a thread to an XML file with stylesheet processing instruction.
func WriteThread(path string, t *Thread) error {
	data, err := MarshalThread(t)
	if err != nil {
		return fmt.Errorf("marshaling thread: %w", err)
	}
	return deleter.WriteAtomic(path, data, 0o644)
}

// MarshalThread marshals a thread to XML bytes with prolog and stylesheet PI.
func MarshalThread(t *Thread) ([]byte, error) {
	body, err := xml.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, err
	}

	var buf strings.Builder
	buf.WriteString(xml.Header)
	buf.WriteString(`<?xml-stylesheet type="text/xsl" href="nota.xslt"?>`)
	buf.WriteByte('\n')
	buf.Write(body)
	buf.WriteByte('\n')

	return []byte(buf.String()), nil
}

// ReadThread reads and parses an XML thread file.
func ReadThread(path string) (*Thread, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return UnmarshalThread(data, path)
}

// UnmarshalThread parses XML bytes into a Thread.
func UnmarshalThread(data []byte, path string) (*Thread, error) {
	var t Thread
	if err := xml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &t, nil
}

// ValidateThread checks a thread for required fields.
// Returns nil if valid, or an error describing what's missing.
func ValidateThread(t *Thread) error {
	var errs []string

	if t.Status == "" {
		errs = append(errs, "missing required field: status")
	} else if t.Status != "open" && t.Status != "resolved" && t.Status != "wontfix" {
		errs = append(errs, fmt.Sprintf("invalid status %q: must be open, resolved, or wontfix", t.Status))
	}

	if len(t.Comments) == 0 {
		errs = append(errs, "thread must have at least one comment")
	}

	for i, c := range t.Comments {
		if c.ID == "" {
			errs = append(errs, fmt.Sprintf("comment %d: missing required field: id", i))
		}
		if c.Author == "" {
			errs = append(errs, fmt.Sprintf("comment %d: missing required field: author", i))
		}
		if len(c.Bodies) == 0 {
			errs = append(errs, fmt.Sprintf("comment %d: must have at least one body", i))
		}
		for j, b := range c.Bodies {
			if b.Time == "" {
				errs = append(errs, fmt.Sprintf("comment %d body %d: missing required field: time", i, j))
			}
		}
	}

	for i, r := range t.Refs {
		count := 0
		if r.Thread != "" {
			count++
		}
		if r.File != "" {
			count++
		}
		if r.Link != "" {
			count++
		}
		switch count {
		case 0:
			errs = append(errs, fmt.Sprintf("ref %d: must have exactly one of thread, file, or link", i))
		case 1:
			// valid
		default:
			errs = append(errs, fmt.Sprintf("ref %d: must have exactly one of thread, file, or link (has %d)", i, count))
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
