package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

type Handler func(ctx context.Context, params json.RawMessage) (any, error)

type Server struct {
	transport *Transport
	handlers  map[string]Handler
	wg        sync.WaitGroup
}

func NewServer(r io.Reader, w io.Writer) *Server {
	return &Server{
		transport: NewTransport(r, w),
		handlers:  make(map[string]Handler),
	}
}

func (s *Server) Handle(method string, handler Handler) {
	s.handlers[method] = handler
}

func (s *Server) Notify(method string, params any) error {
	return s.transport.WriteNotification(NewNotification(method, params))
}

func (s *Server) Serve(ctx context.Context) error {
	for {
		raw, err := s.transport.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) || ctx.Err() != nil {
				break
			}
			if isReadError(err) {
				break
			}
			s.transport.WriteResponse(ParseError())
			continue
		}

		var req Request
		if err := json.Unmarshal(raw, &req); err != nil {
			s.transport.WriteResponse(ParseError())
			continue
		}

		if req.JSONRPC != Version {
			s.transport.WriteResponse(InvalidRequest(req.ID))
			continue
		}

		if req.Method == "" {
			s.transport.WriteResponse(InvalidRequest(req.ID))
			continue
		}

		s.wg.Add(1)
		go s.dispatch(ctx, req)
	}

	s.wg.Wait()
	return nil
}

func (s *Server) dispatch(ctx context.Context, req Request) {
	defer s.wg.Done()

	handler, ok := s.handlers[req.Method]
	if !ok {
		if !req.IsNotification() {
			s.transport.WriteResponse(MethodNotFound(req.ID, req.Method))
		}
		return
	}

	result, err := handler(ctx, req.Params)
	if req.IsNotification() {
		return
	}

	if err != nil {
		s.transport.WriteResponse(errorToResponse(req.ID, err))
		return
	}

	s.transport.WriteResponse(NewResponse(req.ID, result))
}

func errorToResponse(id json.RawMessage, err error) Response {
	if rpcErr, ok := err.(*RPCError); ok {
		return NewErrorResponse(id, rpcErr.Code, rpcErr.Message)
	}
	return InternalError(id, err.Error())
}

type RPCError struct {
	Code    int
	Message string
}

func (e *RPCError) Error() string {
	return e.Message
}

func NewRPCError(code int, message string) *RPCError {
	return &RPCError{Code: code, Message: message}
}

func isReadError(err error) bool {
	var jsonErr *json.SyntaxError
	if errors.As(err, &jsonErr) {
		return false
	}
	var jsonTypeErr *json.UnmarshalTypeError
	if errors.As(err, &jsonTypeErr) {
		return false
	}
	return true
}
