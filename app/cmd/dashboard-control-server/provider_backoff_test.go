package main

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestProviderBackoffPersistsAndClears(t *testing.T) {
	dash := t.TempDir()
	a := &app{cacheDir: filepath.Join(dash, "cache")}
	a.noteProviderBackoff("weather-openmeteo", errors.New("temporary outage"))
	_, reason, failures, active := a.providerBackoffActive("weather-openmeteo")
	if !active || failures != 1 || reason == "" {
		t.Fatalf("unexpected backoff: active=%v failures=%d reason=%q", active, failures, reason)
	}
	a.clearProviderBackoff("weather-openmeteo")
	if _, _, _, active := a.providerBackoffActive("weather-openmeteo"); active {
		t.Fatal("backoff should clear after success")
	}
}
