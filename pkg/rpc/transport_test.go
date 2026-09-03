package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestTransportReadMessage(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"test"}` + "\n"
	transport := NewTransport(strings.NewReader(input), nil)

	ctx := context.Background()
	msg, err := transport.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("ReadMessage error: %v", err)
	}

	var req Request
	if err := json.Unmarshal(msg, &req); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if req.Method != "test" {
		t.Errorf("method = %q, want test", req.Method)
	}
}

func TestTransportReadMessageContextCancel(t *testing.T) {
	reader := &blockingReader{done: make(chan struct{})}
	t.Cleanup(func() { close(reader.done) })

	transport := NewTransport(reader, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := transport.ReadMessage(ctx)
	if err != context.Canceled {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

type blockingReader struct {
	done chan struct{}
}

func (r *blockingReader) Read(p []byte) (int, error) {
	<-r.done
	return 0, nil
}

func TestTransportWriteMessage(t *testing.T) {
	var buf bytes.Buffer
	transport := NewTransport(nil, &buf)

	resp := NewResponse(json.RawMessage(`1`), "result")
	if err := transport.WriteMessage(resp); err != nil {
		t.Fatalf("WriteMessage error: %v", err)
	}

	output := buf.String()
	if !strings.HasSuffix(output, "\n") {
		t.Error("output should end with newline")
	}

	var decoded Response
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Result != "result" {
		t.Errorf("result = %v, want result", decoded.Result)
	}
}

func TestTransportConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	transport := NewTransport(nil, &buf)

	done := make(chan struct{})
	for i := range 10 {
		go func(id int) {
			if err := transport.WriteMessage(NewResponse(json.RawMessage(`1`), id)); err != nil {
				t.Errorf("WriteMessage(%d) error: %v", id, err)
			}
			done <- struct{}{}
		}(i)
	}

	for range 10 {
		<-done
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 10 {
		t.Errorf("got %d lines, want 10", len(lines))
	}

	for _, line := range lines {
		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Errorf("line not valid JSON: %s", line)
		}
	}
}
