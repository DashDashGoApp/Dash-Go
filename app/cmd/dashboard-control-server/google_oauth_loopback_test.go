package main

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"
)

// The server's own wait must outlast the client's poll deadline. When both were
// time.Second, a loaded race-instrumented runner could lose the race to the
// deferred listener close and report a spurious "connection refused" even
// though the loopback path was correct.
const (
	loopbackTestServerWait   = time.Minute
	loopbackTestPollDeadline = 2 * time.Second
)

func TestGoogleOAuthLoopbackWaitReceivesMatchingCallback(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan struct {
		code string
		err  error
	}, 1)
	go func() {
		code, waitErr := googleOAuthLoopbackWaitWith(listener, "expected", "", "family", "https://accounts.example/authorize", false, loopbackTestServerWait, make(chan os.Signal))
		result <- struct {
			code string
			err  error
		}{code, waitErr}
	}()
	callback := "http://" + listener.Addr().String() + "/?code=authorization-code&state=expected"
	deadline := time.Now().Add(loopbackTestPollDeadline)
	for {
		response, requestErr := http.Get(callback)
		if requestErr == nil {
			_ = response.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("loopback callback never became available: %v", requestErr)
		}
		time.Sleep(time.Millisecond)
	}
	out := <-result
	if out.err != nil || out.code != "authorization-code" {
		t.Fatalf("loopback result=%q err=%v", out.code, out.err)
	}
}

func TestGoogleOAuthLoopbackRejectsWrongState(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	interrupted := make(chan os.Signal, 1)
	result := make(chan error, 1)
	go func() {
		_, waitErr := googleOAuthLoopbackWaitWith(listener, "expected", "", "family", "https://accounts.example/authorize", false, loopbackTestServerWait, interrupted)
		result <- waitErr
	}()
	callback := "http://" + listener.Addr().String() + "/?" + url.Values{"code": {"authorization-code"}, "state": {"other"}}.Encode()
	deadline := time.Now().Add(loopbackTestPollDeadline)
	for {
		response, requestErr := http.Get(callback)
		if requestErr == nil {
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("wrong-state status=%d, want 400", response.StatusCode)
			}
			_ = response.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("loopback callback never became available: %v", requestErr)
		}
		time.Sleep(time.Millisecond)
	}
	interrupted <- os.Interrupt
	if err := <-result; !errors.Is(err, errGoogleOAuthInterrupted) {
		t.Fatalf("wrong-state loopback error=%v", err)
	}
}
