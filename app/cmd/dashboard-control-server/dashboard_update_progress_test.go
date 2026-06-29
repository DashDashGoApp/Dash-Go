package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateProgressRouteIsLoopbackAndLightweight(t *testing.T) {
	a := testProfileApp(t)
	if err := writeJSONPrivateFile(a.updateJobPath(), map[string]any{
		"state": "running", "label": "Running update", "detail": "Downloading release.", "source": "control",
	}); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/update/progress", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()
	a.handle(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("progress route status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var got map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["state"] != "running" || got["active"] != true {
		t.Fatalf("progress response=%#v", got)
	}
	for _, forbidden := range []string{"preflight", "availability", "backups", "updaterUnit"} {
		if _, ok := got[forbidden]; ok {
			t.Fatalf("progress route leaked full readiness work %q: %#v", forbidden, got)
		}
	}
}
