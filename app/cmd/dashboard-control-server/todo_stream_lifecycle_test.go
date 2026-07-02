package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type todoStreamTestWriter struct {
	header    http.Header
	mu        sync.Mutex
	body      bytes.Buffer
	deadlines []time.Time
	status    int
	flushes   int
}

func newTodoStreamTestWriter() *todoStreamTestWriter {
	return &todoStreamTestWriter{header: make(http.Header)}
}

func (w *todoStreamTestWriter) Header() http.Header { return w.header }
func (w *todoStreamTestWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		w.status = code
	}
}
func (w *todoStreamTestWriter) Write(body []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(body)
}
func (w *todoStreamTestWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.flushes++
}
func (w *todoStreamTestWriter) SetWriteDeadline(deadline time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.deadlines = append(w.deadlines, deadline)
	return nil
}
func (w *todoStreamTestWriter) snapshot() (string, []time.Time, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String(), append([]time.Time(nil), w.deadlines...), w.flushes
}

func TestTodoSyncExtendsOnlyItsResponseWriteDeadline(t *testing.T) {
	a := newTodoTestApp(t)
	writer := newTodoStreamTestWriter()
	request := httptest.NewRequest(http.MethodPost, "/api/todo/sync", nil)
	before := time.Now()
	if !a.handleTodoPost(writer, request, "/api/todo/sync", map[string]any{}) {
		t.Fatal("sync route was not handled")
	}
	_, deadlines, _ := writer.snapshot()
	if len(deadlines) != 1 {
		t.Fatalf("deadline calls=%d, want one sync-specific override", len(deadlines))
	}
	remaining := deadlines[0].Sub(before)
	if remaining < todoSyncResponseWriteWindow-time.Second || remaining > todoSyncResponseWriteWindow+time.Second {
		t.Fatalf("sync deadline remaining=%s, want about %s", remaining, todoSyncResponseWriteWindow)
	}
}

func TestTodoStreamClearsDeadlineAndEmitsHeartbeat(t *testing.T) {
	a := newTodoTestApp(t)
	previous := todoSSEHeartbeatInterval
	todoSSEHeartbeatInterval = 5 * time.Millisecond
	t.Cleanup(func() { todoSSEHeartbeatInterval = previous })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := newTodoStreamTestWriter()
	request := httptest.NewRequest(http.MethodGet, "/api/todo/stream", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		a.handleTodoStream(writer, request)
		close(done)
	}()

	deadline := time.After(time.Second)
	for {
		body, _, _ := writer.snapshot()
		if bytes.Contains([]byte(body), []byte("event: sync.state")) && bytes.Contains([]byte(body), []byte(": ping\n\n")) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("stream did not emit initial state and heartbeat: %q", body)
		case <-time.After(2 * time.Millisecond):
		}
	}
	_, deadlines, flushes := writer.snapshot()
	if len(deadlines) != 1 || !deadlines[0].IsZero() {
		t.Fatalf("stream deadlines=%#v, want one cleared deadline", deadlines)
	}
	if flushes < 2 {
		t.Fatalf("stream flushes=%d, want initial state plus heartbeat", flushes)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stream did not stop after request cancellation")
	}
}
