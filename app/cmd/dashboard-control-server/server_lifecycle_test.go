package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeHTTPUntilSignalDrainsInflightRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serveHTTPUntilSignal(ctx, srv, listener, time.Second) }()
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		response, requestErr := http.Get("http://" + listener.Addr().String())
		if requestErr == nil {
			_ = response.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request never reached the server")
	}

	cancel()
	select {
	case err := <-done:
		t.Fatalf("server returned before the in-flight request drained: %v", err)
	case <-time.After(60 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("graceful shutdown returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not finish after the request completed")
	}
	select {
	case <-requestDone:
	case <-time.After(time.Second):
		t.Fatal("client did not receive the drained response")
	}
}

func TestServeHTTPUntilSignalRejectsNilInputs(t *testing.T) {
	if err := serveHTTPUntilSignal(context.Background(), nil, nil, time.Second); err == nil {
		t.Fatal("nil server/listener unexpectedly succeeded")
	}
}
