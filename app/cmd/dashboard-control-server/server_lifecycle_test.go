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

// A grace period that expires mid-shutdown must still end as a clean stop,
// never a fatal error — this is the regression shape behind the historical
// "context deadline exceeded → status=1/FAILURE" unit restart.
func TestServeHTTPUntilSignalGraceExpiryIsCleanStop(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-release
	})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serveHTTPUntilSignal(ctx, srv, listener, 50*time.Millisecond) }()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	// Give the server a moment to accept the request, then start shutdown.
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("grace-expiry shutdown returned %v; a planned stop must exit cleanly", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown never completed after grace expiry")
	}
	close(release)
}

// A long-lived stream (SSE) never becomes idle on its own. The release hook must
// end it so a planned stop drains promptly, and the drain must still be a clean
// stop rather than a fatal error.
func TestServeHTTPUntilSignalReleasesLongLivedStreams(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	srv := &http.Server{Handler: http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
	})}
	srv.RegisterOnShutdown(func() { close(release) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serveHTTPUntilSignal(ctx, srv, listener, 30*time.Second) }()
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			_ = response.Body.Close()
		}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("long-lived handler never started")
	}

	began := time.Now()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("released stream produced %v; a planned stop must exit cleanly", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown waited out its grace period despite the release hook")
	}
	if elapsed := time.Since(began); elapsed > 2*time.Second {
		t.Fatalf("drain took %v; expected a prompt stop once the stream was released", elapsed)
	}
}
