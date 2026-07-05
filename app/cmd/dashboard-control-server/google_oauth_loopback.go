package main

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"
)

type googleOAuthLoopbackReply struct {
	code string
	err  error
}

// googleOAuthLoopbackWait owns a short-lived local listener for a Desktop-app
// OAuth redirect. When setup is reached over SSH, an administrator can forward
// their browser's localhost:8433 to this listener with one printed ssh command.
func googleOAuthLoopbackWait(state, displayDir, connection, authorizationURL string, display bool) (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:8433")
	if err != nil {
		return "", fmt.Errorf("could not open temporary local port 8433: %w", err)
	}
	return googleOAuthLoopbackWaitWith(listener, state, displayDir, connection, authorizationURL, display, oauthDisplayWaitLimit, nil)
}

func googleOAuthLoopbackWaitWith(listener net.Listener, state, displayDir, connection, authorizationURL string, display bool, wait time.Duration, interrupt <-chan os.Signal) (string, error) {
	if listener == nil || strings.TrimSpace(state) == "" {
		return "", errors.New("local Google callback is unavailable")
	}
	defer listener.Close()
	var paths oauthDisplayPaths
	presentationArmed := false
	if display && strings.TrimSpace(displayDir) != "" {
		paths = newOAuthDisplayPaths(displayDir)
		presentationArmed = googleOAuthDisplayOnly(paths, state, connection, authorizationURL, time.Now())
		if presentationArmed {
			defer cleanupOAuthDisplay(paths)
		}
	}
	_ = presentationArmed
	result := make(chan googleOAuthLoopbackReply, 1)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/" {
			http.NotFound(w, request)
			return
		}
		query := request.URL.Query()
		if googleErr := strings.TrimSpace(query.Get("error")); googleErr != "" {
			select {
			case result <- googleOAuthLoopbackReply{err: fmt.Errorf("Google returned an error: %s", googleErr)}:
			default:
			}
			http.Error(w, "Google sign-in was not approved. Return to Dash-Go setup.", http.StatusBadRequest)
			return
		}
		code := strings.TrimSpace(query.Get("code"))
		receivedState := strings.TrimSpace(query.Get("state"))
		if code == "" || subtle.ConstantTimeCompare([]byte(receivedState), []byte(state)) != 1 {
			http.Error(w, "This sign-in response does not match the current Dash-Go setup.", http.StatusBadRequest)
			return
		}
		select {
		case result <- googleOAuthLoopbackReply{code: code}:
		default:
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><meta name=\"viewport\" content=\"width=device-width\"><title>Dash-Go connected</title><h2>Dash-Go: Google Calendar connected.</h2><p>You can close this tab and return to setup.</p>"))
	})}
	serveDone := make(chan struct{})
	go func() { _ = server.Serve(listener); close(serveDone) }()
	defer func() { _ = server.Close(); <-serveDone }()
	if interrupt == nil {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, runtimeTerminationSignals()...)
		defer signal.Stop(signals)
		interrupt = signals
	}
	if wait <= 0 || wait > oauthDisplayTTL {
		wait = oauthDisplayWaitLimit
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case reply := <-result:
		return reply.code, reply.err
	case <-interrupt:
		return "", errGoogleOAuthInterrupted
	case <-timer.C:
		return "", fmt.Errorf("no local Google sign-in was received within %s", wait.Round(time.Second))
	}
}
