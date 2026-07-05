package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONSetPathBuildsNestedObject(t *testing.T) {
	root := map[string]any{}
	if err := jsonSetPath(root, "todo.map.grocery", "local-grocery"); err != nil {
		t.Fatalf("jsonSetPath: %v", err)
	}
	todo, ok := root["todo"].(map[string]any)
	if !ok {
		t.Fatalf("todo was not an object: %#v", root)
	}
	mapping, ok := todo["map"].(map[string]any)
	if !ok || mapping["grocery"] != "local-grocery" {
		t.Fatalf("nested value not written: %#v", root)
	}
}

func TestJSONSetCLIWritesObject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"schema":1}`), 0600); err != nil {
		t.Fatal(err)
	}
	var a app
	if code := a.runJSONSetCLI([]string{path, "todo.syncMode", `"microsoft"`}); code != 0 {
		t.Fatalf("runJSONSetCLI returned %d", code)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	todo, ok := got["todo"].(map[string]any)
	if !ok || todo["syncMode"] != "microsoft" {
		t.Fatalf("json-set did not preserve/write nested field: %#v", got)
	}
}

func TestGeocodeCLIUsesEncodedEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("name"); got != "Zürich" {
			t.Errorf("name query = %q, want Zürich", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"name":"Zürich","admin1":"Zürich","country":"Switzerland","latitude":47.3769,"longitude":8.5417}]}`))
	}))
	defer server.Close()
	t.Setenv("DASH_GO_GEOCODE_ENDPOINT", server.URL)

	old := os.Stdout
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = write
	defer func() { os.Stdout = old }()
	var a app
	code := a.runGeocodeCLI([]string{"--name", "Zürich"})
	_ = write.Close()
	output, _ := io.ReadAll(read)
	_ = read.Close()
	if code != 0 {
		t.Fatalf("runGeocodeCLI returned %d", code)
	}
	if !strings.Contains(string(output), "47.3769|8.5417|Zürich, Zürich, Switzerland") {
		t.Fatalf("unexpected geocode output: %q", output)
	}
}

func TestInstallerNormalizeAppVisibilityCLI(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	configPath := filepath.Join(dir, "config.local.js")
	settings := `{"showChalkboard":false,"radarEnabled":false,"todo":{"enabled":false,"map":{"todo":"","grocery":"local-grocery"}}}`
	if err := os.WriteFile(settingsPath, []byte(settings), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("const config={\nshowChalkboard:false,\nradarEnabled:false,\nkeep:true};\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var a app
	if code := a.runInstallerNormalizeAppVisibilityCLI([]string{"--settings", settingsPath, "--config-local", configPath}); code != 0 {
		t.Fatalf("runInstallerNormalizeAppVisibilityCLI returned %d", code)
	}
	body, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["showChalkboard"]; ok {
		t.Fatalf("showChalkboard survived cleanup: %#v", got)
	}
	if _, ok := got["radarEnabled"]; ok {
		t.Fatalf("radarEnabled survived cleanup: %#v", got)
	}
	todo := got["todo"].(map[string]any)
	if _, ok := todo["enabled"]; ok {
		t.Fatalf("todo.enabled survived cleanup: %#v", todo)
	}
	mapping := todo["map"].(map[string]any)
	if mapping["todo"] != "local-todo" || mapping["grocery"] != "local-grocery" {
		t.Fatalf("unexpected default mappings: %#v", mapping)
	}
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(config), "showChalkboard") || strings.Contains(string(config), "radarEnabled") || !strings.Contains(string(config), "keep:true") {
		t.Fatalf("unexpected config cleanup result: %s", config)
	}
}

func TestJSONSetStringCLIPreservesJSONLookingText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var a app
	if code := a.runJSONSetStringCLI([]string{path, "identity.code", "0420"}); code != 0 {
		t.Fatalf("runJSONSetStringCLI returned %d", code)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	identity, ok := got["identity"].(map[string]any)
	if !ok || identity["code"] != "0420" {
		t.Fatalf("string mode retyped a JSON-looking value: %#v", got)
	}
}

func TestJSONSetPathRefusesNonObjectIntermediate(t *testing.T) {
	root := map[string]any{"todo": "legacy-string"}
	if err := jsonSetPath(root, "todo.syncMode", "local"); err == nil {
		t.Fatalf("jsonSetPath overwrote a non-object intermediate: %#v", root)
	}
	if root["todo"] != "legacy-string" {
		t.Fatalf("jsonSetPath changed the protected intermediate: %#v", root)
	}
}
