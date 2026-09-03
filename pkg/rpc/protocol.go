package rpc

import "encoding/json"

const (
	Version = "2.0"

	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
	CodeNotFound       = -32000
)

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
	ID      json.RawMessage `json:"id"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type Notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

func (r *Request) IsNotification() bool {
	return r.ID == nil || string(r.ID) == "null"
}

func NewResponse(id json.RawMessage, result any) Response {
	return Response{
		JSONRPC: Version,
		Result:  result,
		ID:      id,
	}
}

func NewErrorResponse(id json.RawMessage, code int, message string) Response {
	return Response{
		JSONRPC: Version,
		Error:   &Error{Code: code, Message: message},
		ID:      id,
	}
}

func NewNotification(method string, params any) Notification {
	return Notification{
		JSONRPC: Version,
		Method:  method,
		Params:  params,
	}
}

func ParseError() Response {
	return NewErrorResponse(nil, CodeParseError, "Parse error")
}

func InvalidRequest(id json.RawMessage) Response {
	return NewErrorResponse(id, CodeInvalidRequest, "Invalid Request")
}

func MethodNotFound(id json.RawMessage, method string) Response {
	return NewErrorResponse(id, CodeMethodNotFound, "Method not found: "+method)
}

func InvalidParams(id json.RawMessage, msg string) Response {
	return NewErrorResponse(id, CodeInvalidParams, msg)
}

func InternalError(id json.RawMessage, msg string) Response {
	return NewErrorResponse(id, CodeInternalError, msg)
}

func NotFoundError(id json.RawMessage, msg string) Response {
	return NewErrorResponse(id, CodeNotFound, msg)
}
