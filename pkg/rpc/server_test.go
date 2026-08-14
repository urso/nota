package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestServerDispatch(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"echo","params":{"msg":"hello"},"id":1}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	server := NewServer(reader, &output)
	server.Handle("echo", func(ctx context.Context, params json.RawMessage) (any, error) {
		var p struct{ Msg string }
		json.Unmarshal(params, &p)
		return map[string]string{"echo": p.Msg}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	server.Serve(ctx)

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("unexpected error: %+v", resp.Error)
	}

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", resp.Result)
	}
	if result["echo"] != "hello" {
		t.Errorf("result[echo] = %v, want hello", result["echo"])
	}
}

func TestServerMethodNotFound(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"unknown","id":"abc"}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	server := NewServer(reader, &output)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	server.Serve(ctx)

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != CodeMethodNotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, CodeMethodNotFound)
	}
}

func TestServerParseError(t *testing.T) {
	input := `{invalid json}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	server := NewServer(reader, &output)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	server.Serve(ctx)

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != CodeParseError {
		t.Errorf("error code = %d, want %d", resp.Error.Code, CodeParseError)
	}
}

func TestServerInvalidRequest(t *testing.T) {
	input := `{"jsonrpc":"1.0","method":"test","id":1}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	server := NewServer(reader, &output)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	server.Serve(ctx)

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != CodeInvalidRequest {
		t.Errorf("error code = %d, want %d", resp.Error.Code, CodeInvalidRequest)
	}
}

func TestServerConcurrentRequests(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"slow","id":1}
{"jsonrpc":"2.0","method":"fast","id":2}
`
	reader := strings.NewReader(input)
	var output bytes.Buffer

	server := NewServer(reader, &output)

	slowDone := make(chan struct{})
	fastDone := make(chan struct{})
	server.Handle("slow", func(ctx context.Context, params json.RawMessage) (any, error) {
		<-slowDone
		return "slow", nil
	})
	server.Handle("fast", func(ctx context.Context, params json.RawMessage) (any, error) {
		close(fastDone)
		return "fast", nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		server.Serve(ctx)
		close(done)
	}()

	<-fastDone
	close(slowDone)

	<-done

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d responses, want 2", len(lines))
	}

	var ids []int
	for _, line := range lines {
		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		var id int
		if err := json.Unmarshal(resp.ID, &id); err != nil {
			t.Fatalf("unmarshal id: %v", err)
		}
		ids = append(ids, id)
	}

	if ids[0] != 2 {
		t.Errorf("first response id = %d, want 2 (fast should complete first)", ids[0])
	}
}

func TestServerNotification(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"notify"}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	called := false
	server := NewServer(reader, &output)
	server.Handle("notify", func(ctx context.Context, params json.RawMessage) (any, error) {
		called = true
		return "ignored", nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	server.Serve(ctx)

	if !called {
		t.Error("handler was not called")
	}
	if output.Len() != 0 {
		t.Errorf("notification should not produce response, got: %s", output.String())
	}
}

func TestServerNotify(t *testing.T) {
	var output bytes.Buffer
	server := NewServer(strings.NewReader(""), &output)

	err := server.Notify("thread/didChange", map[string][]string{
		"threadIds": {"id1"},
		"files":     {"file.go"},
	})
	if err != nil {
		t.Fatalf("Notify error: %v", err)
	}

	var notif Notification
	if err := json.Unmarshal(output.Bytes(), &notif); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if notif.Method != "thread/didChange" {
		t.Errorf("method = %q, want thread/didChange", notif.Method)
	}
}

func TestServerReadErrorStopsServing(t *testing.T) {
	reader := &failingReader{err: errors.New("connection reset")}
	var output bytes.Buffer

	server := NewServer(reader, &output)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	server.Serve(ctx)

	if output.Len() != 0 {
		t.Errorf("expected no response on read error, got: %s", output.String())
	}
}

type failingReader struct {
	err error
}

func (r *failingReader) Read(p []byte) (int, error) {
	return 0, r.err
}

func TestServerRPCError(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"fail","id":1}` + "\n"
	reader := strings.NewReader(input)
	var output bytes.Buffer

	server := NewServer(reader, &output)
	server.Handle("fail", func(ctx context.Context, params json.RawMessage) (any, error) {
		return nil, NewRPCError(CodeNotFound, "not found")
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	server.Serve(ctx)

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != CodeNotFound {
		t.Errorf("code = %d, want %d", resp.Error.Code, CodeNotFound)
	}
}
