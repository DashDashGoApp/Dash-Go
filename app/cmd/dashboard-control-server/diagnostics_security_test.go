package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDiagnosticsBundleUsesPrivateHomeDirectory(t *testing.T) {
	a := testApp(t)
	legacy := filepath.Join(a.cacheDir, diagnosticsBundleName)
	if err := os.WriteFile(legacy, []byte("legacy bundle"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := a.buildDiagnosticsWithHealth(map[string]any{"ok": true, "outputTail": "healthy"})
	if err != nil {
		t.Fatalf("build diagnostics: %v", err)
	}
	path := filepath.Join(a.diagnosticsDir(), diagnosticsBundleName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("private bundle missing: %v", err)
	}
	if filepath.Dir(path) == a.cacheDir {
		t.Fatal("diagnostics bundle remained in static cache directory")
	}
	if got := result["location"]; got != a.diagnosticsLocationHint() {
		t.Fatalf("location=%q want %q", got, a.diagnosticsLocationHint())
	}
	if st, err := os.Stat(a.diagnosticsDir()); err != nil || st.Mode().Perm() != 0700 {
		t.Fatalf("diagnostics directory mode=%v err=%v want 0700", func() any {
			if st == nil {
				return nil
			}
			return st.Mode().Perm()
		}(), err)
	}
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0600 {
		t.Fatalf("diagnostics bundle mode=%v err=%v want 0600", func() any {
			if st == nil {
				return nil
			}
			return st.Mode().Perm()
		}(), err)
	}

	w := httptest.NewRecorder()
	a.handle(w, httptest.NewRequest(http.MethodGet, "/cache/"+diagnosticsBundleName, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("legacy static diagnostics status=%d want 404", w.Code)
	}
}
