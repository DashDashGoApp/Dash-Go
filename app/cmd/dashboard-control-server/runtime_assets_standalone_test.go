package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

// The release builder invokes runtime_assets.go explicitly with a tiny helper
// main. Explicit file lists bypass the rest of package main, so this test keeps
// that contract visible when shared helpers are refactored.
func TestRuntimeAssetsStandaloneVerifierRuns(t *testing.T) {
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(goBinary); err != nil {
		t.Skip("selected Go toolchain executable is unavailable: " + err.Error())
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate runtime assets test")
	}
	serverRoot := filepath.Dir(thisFile)
	projectRoot := filepath.Clean(filepath.Join(serverRoot, "..", ".."))
	version := strings.TrimSpace(fileio.ReadString(filepath.Join(projectRoot, "VERSION"), ""))
	if version == "" {
		t.Fatal("missing source version")
	}

	work := t.TempDir()
	runtimeAssets, err := os.ReadFile(filepath.Join(serverRoot, "runtime_assets.go"))
	if err != nil {
		t.Fatal(err)
	}
	writeStandaloneFile(t, filepath.Join(work, "go.mod"), []byte("module github.com/DashDashGoApp/Dash-Go/app\n\ngo 1.26\n\ntoolchain go1.26.4\n"))
	writeStandaloneFile(t, filepath.Join(work, "runtime_assets.go"), runtimeAssets)
	writeStandaloneFile(t, filepath.Join(work, "verify_main.go"), []byte(standaloneRuntimeAssetsVerifyMain))
	for _, rel := range []string{"internal/fileio/fileio.go", "internal/jsonutil/jsonutil.go"} {
		body, err := os.ReadFile(filepath.Join(projectRoot, rel))
		if err != nil {
			t.Fatalf("read standalone helper %s: %v", rel, err)
		}
		writeStandaloneFile(t, filepath.Join(work, rel), body)
	}

	fixture := filepath.Join(work, "source")
	writeStandaloneFile(t, filepath.Join(fixture, "VERSION"), []byte(version+"\n"))
	writeStandaloneFile(t, filepath.Join(fixture, "index.html"), []byte("<link href=\"ui/dashboard.css?v="+version+"\">\n<script src=\"ui/js/app.bundle.js?v="+version+"\"></script>\n"))
	writeStandaloneFile(t, filepath.Join(fixture, "ui", "js", "config-defaults.js"), []byte("const CONFIG={version: \""+version+"\"};\n"))
	writeStandaloneFile(t, filepath.Join(fixture, "ui", "js", "control-lazy-loader.js"), []byte("controlAssetURL(\"ui/control-layout.css\"); controlAssetURL(\"ui/js/app.control.bundle.js\");\n"))
	writeStandaloneFile(t, filepath.Join(fixture, "ui", "js", "dashboard-core.js"), []byte("window.dashboard=true;\n"))
	writeStandaloneFile(t, filepath.Join(fixture, "ui", "js", "control-test.js"), []byte("window.control=true;\n"))
	writeStandaloneFile(t, filepath.Join(fixture, "ui", "js", "bundle.manifest.json"), []byte(`{"schema":1,"bundles":{"app":["dashboard-core.js"],"control":["control-test.js"]}}
`))
	writeStandaloneFile(t, filepath.Join(fixture, "ui", "css", "dashboard", "base.css"), []byte("body{}\n"))
	writeStandaloneFile(t, filepath.Join(fixture, "ui", "css", "control", "00-core.css"), []byte(".control{}\n"))
	writeStandaloneFile(t, filepath.Join(fixture, "ui", "css", "bundle.manifest.json"), []byte(`{"schema":1,"bundles":{"dashboard":["dashboard/base.css"],"control":["control/00-core.css"]}}
`))

	if err := verifyGeneratedAssets(fixture, true); err != nil {
		t.Fatalf("prepare generated fixture: %v", err)
	}
	cmd := exec.Command(goBinary, "run", ".", fixture)
	cmd.Dir = work
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("standalone generated-assets verifier failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "generated assets are current") {
		t.Fatalf("standalone verifier did not report success: %s", output)
	}
}

func writeStandaloneFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatal(err)
	}
}

const standaloneRuntimeAssetsVerifyMain = `package main

import (
	"fmt"
	"os"
)

type app struct{ dash string }

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: verify_main <source-root>")
		os.Exit(2)
	}
	if err := verifyGeneratedAssets(os.Args[1], false); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("generated assets are current")
}
`
