package service

import "github.com/urso/nota/pkg/thread"

// ThreadView pairs a thread with its resolved anchor and full title.
type ThreadView struct {
	Thread         *thread.Thread
	ResolvedAnchor *ResolvedAnchor
	Title          string
}

// ResolvedAnchor holds the working-tree position of a thread's anchor.
type ResolvedAnchor struct {
	File     string
	Line     int
	Outdated bool
}

// ChangeEvent identifies threads and files affected by a change.
type ChangeEvent struct {
	ThreadIDs []string
	Files     []string
}

// Filter specifies criteria for listing threads.
type Filter struct {
	Status string
	Goal   string
	Group  string
	Tag    string
	File   string // matches resolved anchor path (repo-relative)
}
