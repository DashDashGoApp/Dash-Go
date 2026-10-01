package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"time"
)

// serveHTTPUntilSignal owns only process-level server lifecycle. Requests keep
// their ordinary handler deadlines; a SIGTERM/SIGINT starts a bounded graceful
// shutdown so systemd update/restart work can drain in-flight responses.
func serveHTTPUntilSignal(ctx context.Context, srv *http.Server, listener net.Listener, shutdownTimeout time.Duration) error {
	if srv == nil {
		return errors.New("dashboard HTTP server is nil")
	}
	if listener == nil {
		return errors.New("dashboard HTTP listener is nil")
	}
	if shutdownTimeout <= 0 {
		shutdownTimeout = 8 * time.Second
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(listener) }()

	select {
	case err := <-errCh:
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			// A grace period that expires is a normal restart condition, not a
			// fault: long-lived streams and slow clients can outlast the window.
			// Release the remaining connections and report a clean stop, so a
			// planned update/restart is never recorded as a failed unit.
			log.Printf("dashboard shutdown grace expired (%v); closing remaining connections", err)
			_ = srv.Close()
		}
		err := <-errCh
		if err == nil || errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// beginShutdown releases long-lived streams so a planned stop drains promptly
// instead of waiting out the full shutdown grace period. Safe to call twice.
func (a *app) beginShutdown() {
	a.shutdownOnce.Do(func() {
		if a.shutdownCh != nil {
			close(a.shutdownCh)
		}
	})
}
