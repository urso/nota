package rpc

import (
	"context"
	"encoding/json"

	"github.com/urso/nota/pkg/service"
)

type ThreadHandlers struct {
	svc *service.Service
}

func NewThreadHandlers(svc *service.Service) *ThreadHandlers {
	return &ThreadHandlers{svc: svc}
}

func (h *ThreadHandlers) Register(s *Server) {
	s.Handle("thread/list", h.List)
	s.Handle("thread/get", h.Get)
	s.Handle("thread/create", h.Create)
	s.Handle("thread/addComment", h.AddComment)
	s.Handle("thread/setStatus", h.SetStatus)
}

type ListParams struct {
	Status string `json:"status,omitempty"`
	Goal   string `json:"goal,omitempty"`
	Group  string `json:"group,omitempty"`
	Tag    string `json:"tag,omitempty"`
	File   string `json:"file,omitempty"`
}

func (h *ThreadHandlers) List(ctx context.Context, params json.RawMessage) (any, error) {
	var p ListParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, NewRPCError(CodeInvalidParams, "invalid params: "+err.Error())
		}
	}

	views, err := h.svc.List(service.Filter{
		Status: p.Status,
		Goal:   p.Goal,
		Group:  p.Group,
		Tag:    p.Tag,
		File:   p.File,
	})
	if err != nil {
		return nil, err
	}

	result := make([]service.ThreadViewJSON, len(views))
	for i, v := range views {
		result[i] = v.ToJSON()
	}
	return result, nil
}

type GetParams struct {
	ID string `json:"id"`
}

func (h *ThreadHandlers) Get(ctx context.Context, params json.RawMessage) (any, error) {
	var p GetParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewRPCError(CodeInvalidParams, "invalid params: "+err.Error())
	}

	if p.ID == "" {
		return nil, NewRPCError(CodeInvalidParams, "id is required")
	}

	view, err := h.svc.Get(p.ID)
	if err != nil {
		return nil, err
	}
	if view == nil {
		return nil, NewRPCError(CodeNotFound, "thread not found: "+p.ID)
	}

	return view.ToJSON(), nil
}

type CreateParams struct {
	Message string `json:"message"`
	Body    string `json:"body,omitempty"`
	Goal    string `json:"goal,omitempty"`
	Group   string `json:"group,omitempty"`
	Tags    string `json:"tags,omitempty"`
	Parent  string `json:"parent,omitempty"`
	Anchor  string `json:"anchor,omitempty"`
	Author  string `json:"author,omitempty"`
}

func (h *ThreadHandlers) Create(ctx context.Context, params json.RawMessage) (any, error) {
	var p CreateParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewRPCError(CodeInvalidParams, "invalid params: "+err.Error())
	}

	if p.Message == "" {
		return nil, NewRPCError(CodeInvalidParams, "message is required")
	}

	view, err := h.svc.Create(service.CreateOpts{
		Message: p.Message,
		Body:    p.Body,
		Goal:    p.Goal,
		Group:   p.Group,
		Tags:    p.Tags,
		Parent:  p.Parent,
		Anchor:  p.Anchor,
		Author:  p.Author,
	})
	if err != nil {
		return nil, err
	}

	return view.ToJSON(), nil
}

type AddCommentParams struct {
	ThreadID string `json:"threadId"`
	Message  string `json:"message"`
	Author   string `json:"author,omitempty"`
	Local    bool   `json:"local,omitempty"`
	ReplyTo  string `json:"replyTo,omitempty"`
	Anchor   string `json:"anchor,omitempty"`
}

func (h *ThreadHandlers) AddComment(ctx context.Context, params json.RawMessage) (any, error) {
	var p AddCommentParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewRPCError(CodeInvalidParams, "invalid params: "+err.Error())
	}

	if p.ThreadID == "" {
		return nil, NewRPCError(CodeInvalidParams, "threadId is required")
	}
	if p.Message == "" {
		return nil, NewRPCError(CodeInvalidParams, "message is required")
	}

	view, err := h.svc.AddComment(p.ThreadID, service.AddCommentOpts{
		Message: p.Message,
		Author:  p.Author,
		Local:   p.Local,
		ReplyTo: p.ReplyTo,
		Anchor:  p.Anchor,
	})
	if err != nil {
		return nil, err
	}

	return view.ToJSON(), nil
}

type SetStatusParams struct {
	ThreadID string `json:"threadId"`
	Status   string `json:"status"`
}

func (h *ThreadHandlers) SetStatus(ctx context.Context, params json.RawMessage) (any, error) {
	var p SetStatusParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, NewRPCError(CodeInvalidParams, "invalid params: "+err.Error())
	}

	if p.ThreadID == "" {
		return nil, NewRPCError(CodeInvalidParams, "threadId is required")
	}
	if p.Status == "" {
		return nil, NewRPCError(CodeInvalidParams, "status is required")
	}

	view, err := h.svc.SetStatus(p.ThreadID, p.Status)
	if err != nil {
		return nil, err
	}

	return view.ToJSON(), nil
}

type DidChangeParams struct {
	Repo      string   `json:"repo"`
	ThreadIDs []string `json:"threadIds"`
	Files     []string `json:"files"`
}

func SubscribeAndNotify(ctx context.Context, svc *service.Service, server *Server) {
	ch := svc.Subscribe()
	defer svc.Unsubscribe(ch)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			server.Notify("thread/didChange", DidChangeParams{
				Repo:      svc.RepoRoot(),
				ThreadIDs: event.ThreadIDs,
				Files:     event.Files,
			})
		}
	}
}
