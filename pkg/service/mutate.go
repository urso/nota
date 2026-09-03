package service

import (
	"github.com/urso/nota/pkg/thread"
)

// CreateOpts holds options for creating a thread.
type CreateOpts struct {
	Message string
	Body    string
	Goal    string
	Group   string
	Tags    string
	Parent  string
	Anchor  string
	Author  string
}

// Create creates a new thread and returns a ThreadView.
func (s *Service) Create(opts CreateOpts) (*ThreadView, error) {
	t, err := thread.Create(s.notaDir, thread.CreateOpts{
		Message: opts.Message,
		Body:    opts.Body,
		Goal:    opts.Goal,
		Group:   opts.Group,
		Tags:    opts.Tags,
		Parent:  opts.Parent,
		Anchor:  opts.Anchor,
		Author:  opts.Author,
	})
	if err != nil {
		return nil, err
	}

	view := s.buildView(t)
	s.markPendingWrite(t.ID)
	s.notify(ChangeEvent{
		ThreadIDs: []string{t.ID},
		Files:     filesFromThread(t),
	})
	return &view, nil
}

// AddCommentOpts holds options for adding a comment.
type AddCommentOpts struct {
	Message string
	Author  string
	Local   bool
	ReplyTo string
	Anchor  string
}

// AddComment adds a comment to an existing thread.
func (s *Service) AddComment(threadID string, opts AddCommentOpts) (*ThreadView, error) {
	_, err := thread.AddComment(s.notaDir, threadID, thread.AddCommentOpts{
		Message: opts.Message,
		Author:  opts.Author,
		Local:   opts.Local,
		ReplyTo: opts.ReplyTo,
		Anchor:  opts.Anchor,
	})
	if err != nil {
		return nil, err
	}

	view, err := s.Get(threadID)
	if err != nil {
		return nil, err
	}

	s.markPendingWrite(view.Thread.ID)
	s.notify(ChangeEvent{
		ThreadIDs: []string{view.Thread.ID},
		Files:     filesFromThread(view.Thread),
	})
	return view, nil
}

// SetStatus updates a thread's status.
func (s *Service) SetStatus(threadID, status string) (*ThreadView, error) {
	if err := thread.UpdateStatus(s.notaDir, threadID, status); err != nil {
		return nil, err
	}

	view, err := s.Get(threadID)
	if err != nil {
		return nil, err
	}

	s.markPendingWrite(view.Thread.ID)
	s.notify(ChangeEvent{
		ThreadIDs: []string{view.Thread.ID},
		Files:     filesFromThread(view.Thread),
	})
	return view, nil
}

// filesFromThread extracts file paths from a thread's anchors.
func filesFromThread(t *thread.Thread) []string {
	var files []string
	seen := make(map[string]bool)

	for _, a := range t.Anchors {
		if !seen[a.File] {
			seen[a.File] = true
			files = append(files, a.File)
		}
	}
	for _, fa := range t.FileAnchors {
		if !seen[fa.File] {
			seen[fa.File] = true
			files = append(files, fa.File)
		}
	}

	return files
}
