package rpc

import (
	"encoding/json"
	"testing"
)

func TestRequestIsNotification(t *testing.T) {
	tests := []struct {
		name string
		id   json.RawMessage
		want bool
	}{
		{"nil id", nil, true},
		{"null id", json.RawMessage(`null`), true},
		{"string id", json.RawMessage(`"abc"`), false},
		{"number id", json.RawMessage(`1`), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := Request{ID: tt.id}
			if got := req.IsNotification(); got != tt.want {
				t.Errorf("IsNotification() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewResponse(t *testing.T) {
	id := json.RawMessage(`"test-id"`)
	resp := NewResponse(id, map[string]string{"key": "value"})

	if resp.JSONRPC != Version {
		t.Errorf("JSONRPC = %q, want %q", resp.JSONRPC, Version)
	}
	if resp.Error != nil {
		t.Error("Error should be nil")
	}
	if string(resp.ID) != `"test-id"` {
		t.Errorf("ID = %s, want %q", resp.ID, `"test-id"`)
	}
}

func TestNewErrorResponse(t *testing.T) {
	id := json.RawMessage(`42`)
	resp := NewErrorResponse(id, CodeMethodNotFound, "method not found")

	if resp.JSONRPC != Version {
		t.Errorf("JSONRPC = %q, want %q", resp.JSONRPC, Version)
	}
	if resp.Result != nil {
		t.Error("Result should be nil")
	}
	if resp.Error == nil {
		t.Fatal("Error should not be nil")
	}
	if resp.Error.Code != CodeMethodNotFound {
		t.Errorf("Error.Code = %d, want %d", resp.Error.Code, CodeMethodNotFound)
	}
	if resp.Error.Message != "method not found" {
		t.Errorf("Error.Message = %q, want %q", resp.Error.Message, "method not found")
	}
}

func TestNewNotification(t *testing.T) {
	notif := NewNotification("thread/didChange", map[string][]string{
		"threadIds": {"id1", "id2"},
	})

	if notif.JSONRPC != Version {
		t.Errorf("JSONRPC = %q, want %q", notif.JSONRPC, Version)
	}
	if notif.Method != "thread/didChange" {
		t.Errorf("Method = %q, want %q", notif.Method, "thread/didChange")
	}
}

func TestErrorHelpers(t *testing.T) {
	tests := []struct {
		name     string
		resp     Response
		wantCode int
	}{
		{"ParseError", ParseError(), CodeParseError},
		{"InvalidRequest", InvalidRequest(nil), CodeInvalidRequest},
		{"MethodNotFound", MethodNotFound(nil, "foo"), CodeMethodNotFound},
		{"InvalidParams", InvalidParams(nil, "bad"), CodeInvalidParams},
		{"InternalError", InternalError(nil, "oops"), CodeInternalError},
		{"NotFoundError", NotFoundError(nil, "gone"), CodeNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.resp.Error == nil {
				t.Fatal("Error should not be nil")
			}
			if tt.resp.Error.Code != tt.wantCode {
				t.Errorf("Code = %d, want %d", tt.resp.Error.Code, tt.wantCode)
			}
		})
	}
}
