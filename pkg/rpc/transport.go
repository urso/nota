package rpc

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"
)

type Transport struct {
	reader  *bufio.Reader
	writer  io.Writer
	writeMu sync.Mutex
}

func NewTransport(r io.Reader, w io.Writer) *Transport {
	return &Transport{
		reader: bufio.NewReader(r),
		writer: w,
	}
}

func (t *Transport) ReadMessage(ctx context.Context) (json.RawMessage, error) {
	type result struct {
		line []byte
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		line, err := t.reader.ReadBytes('\n')
		ch <- result{line, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, r.err
		}
		return json.RawMessage(r.line), nil
	}
}

func (t *Transport) WriteMessage(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	_, err = t.writer.Write(data)
	return err
}

func (t *Transport) WriteResponse(resp Response) error {
	return t.WriteMessage(resp)
}

func (t *Transport) WriteNotification(notif Notification) error {
	return t.WriteMessage(notif)
}
