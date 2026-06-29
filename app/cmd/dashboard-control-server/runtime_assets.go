package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Keep these verifier-only patterns local. The release builder deliberately
// invokes this file with verify_main.go as a tiny standalone command, so that
// command cannot depend on the dashboard server's broader shared regexp file.
var (
	runtimeAssetWhitespace      = regexp.MustCompile(`\s+`)
	runtimeAssetConfigLatitude  = regexp.MustCompile(`\blat:\s*-?\d`)
	runtimeAssetConfigLongitude = regexp.MustCompile(`\blon:\s*-?\d`)
)

func (a *app) runVerifyGeneratedAssetsCLI(args []string) int {
	write := false
	for _, arg := range args {
		if arg == "--write" {
			write = true
		}
	}
	if err := verifyGeneratedAssets(a.dash, write); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println("generated assets are current")
	return 0
}

const (
	runtimeAssetJSManifestRel  = "ui/js/bundle.manifest.json"
	runtimeAssetCSSManifestRel = "ui/css/bundle.manifest.json"
)

type runtimeAssetManifest struct {
	Schema  int                 `json:"schema"`
	Bundles map[string][]string `json:"bundles"`
}

// runtimeAssetManifestFiles resolves one manifest into ordered source files.
// The manifest is authoritative: split-source filenames no longer determine
// browser load order. Paths are kept relative to their source directory so a
// later semantic rename or folder move changes only manifest entries, not
// bundle construction behavior.
func runtimeAssetManifestFiles(root, manifestRel, sourceRel, ext string, bundleNames ...string) (map[string][]string, error) {
	manifestPath := filepath.Join(root, filepath.FromSlash(manifestRel))
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("read asset manifest %s: %w", manifestRel, err)
	}
	var manifest runtimeAssetManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, fmt.Errorf("parse asset manifest %s: %w", manifestRel, err)
	}
	if manifest.Schema != 1 {
		return nil, fmt.Errorf("asset manifest %s has unsupported schema %d", manifestRel, manifest.Schema)
	}
	if manifest.Bundles == nil {
		return nil, fmt.Errorf("asset manifest %s has no bundles", manifestRel)
	}

	expected := make(map[string]struct{}, len(bundleNames))
	for _, name := range bundleNames {
		expected[name] = struct{}{}
	}
	for name := range manifest.Bundles {
		if _, ok := expected[name]; !ok {
			return nil, fmt.Errorf("asset manifest %s has unknown bundle %q", manifestRel, name)
		}
	}

	sourceRoot := filepath.Join(root, filepath.FromSlash(sourceRel))
	seen := map[string]string{}
	resolved := make(map[string][]string, len(bundleNames))
	for _, bundle := range bundleNames {
		entries, ok := manifest.Bundles[bundle]
		if !ok {
			return nil, fmt.Errorf("asset manifest %s is missing bundle %q", manifestRel, bundle)
		}
		if len(entries) == 0 {
			return nil, fmt.Errorf("asset manifest %s bundle %q is empty", manifestRel, bundle)
		}
		files := make([]string, 0, len(entries))
		for _, entry := range entries {
			path, rel, err := runtimeAssetSourcePath(sourceRoot, entry)
			if err != nil {
				return nil, fmt.Errorf("asset manifest %s bundle %q: %w", manifestRel, bundle, err)
			}
			if filepath.Ext(path) != ext {
				return nil, fmt.Errorf("asset manifest %s bundle %q entry %q must be a %s source", manifestRel, bundle, entry, ext)
			}
			if sourceRel == "ui/js" && (filepath.Base(path) == "app.bundle.js" || filepath.Base(path) == "app.control.bundle.js") {
				return nil, fmt.Errorf("asset manifest %s bundle %q entry %q names a generated JavaScript bundle", manifestRel, bundle, entry)
			}
			if prior, duplicate := seen[rel]; duplicate {
				return nil, fmt.Errorf("asset manifest %s bundle %q entry %q duplicates bundle %q", manifestRel, bundle, entry, prior)
			}
			info, err := os.Lstat(path)
			if err != nil {
				return nil, fmt.Errorf("asset manifest %s bundle %q entry %q: %w", manifestRel, bundle, entry, err)
			}
			if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
				return nil, fmt.Errorf("asset manifest %s bundle %q entry %q is not a regular source file", manifestRel, bundle, entry)
			}
			seen[rel] = bundle
			files = append(files, path)
		}
		resolved[bundle] = files
	}
	return resolved, nil
}

func runtimeAssetSourcePath(sourceRoot, entry string) (string, string, error) {
	if entry == "" || entry != strings.TrimSpace(entry) || strings.Contains(entry, `\`) {
		return "", "", fmt.Errorf("invalid source entry %q", entry)
	}
	clean := filepath.Clean(filepath.FromSlash(entry))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("source entry %q escapes its source directory", entry)
	}
	path := filepath.Join(sourceRoot, clean)
	rel, err := filepath.Rel(sourceRoot, path)
	if err != nil {
		return "", "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("source entry %q escapes its source directory", entry)
	}
	return path, filepath.ToSlash(rel), nil
}

func verifyGeneratedAssets(root string, write bool) error {
	version := strings.TrimSpace(fileio.ReadString(filepath.Join(root, "VERSION"), "unknown"))
	jsBundles, err := runtimeAssetManifestFiles(root, runtimeAssetJSManifestRel, "ui/js", ".js", "app", "control")
	if err != nil {
		return err
	}
	cssBundles, err := runtimeAssetManifestFiles(root, runtimeAssetCSSManifestRel, "ui/css", ".css", "dashboard", "control")
	if err != nil {
		return err
	}
	readNorm := func(path string) string {
		b, _ := os.ReadFile(path)
		s := string(b)
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		return s
	}
	buildJS := func(bundle string) string {
		label := "browser bundle"
		target := "ui/js/app.bundle.js"
		if bundle == "control" {
			label = "control bundle"
			target = "ui/js/app.control.bundle.js"
		}
		var b strings.Builder
		fmt.Fprintf(&b, "/* Dash-Go %s %s. GENERATED as %s from ui/js split source files; edit split files, not this bundle. */\n", version, label, target)
		for _, p := range jsBundles[bundle] {
			rel, _ := filepath.Rel(root, p)
			fmt.Fprintf(&b, "\n/* ===== %s ===== */\n", filepath.ToSlash(rel))
			b.WriteString(readNorm(p))
		}
		return b.String()
	}
	buildCSS := func(bundle, target string) string {
		var b strings.Builder
		fmt.Fprintf(&b, "/* Dash-Go %s %s browser bundle.\n   GENERATED from ui/css/%s split source files; edit split files, not this bundle. */\n", version, target, bundle)
		for _, p := range cssBundles[bundle] {
			rel, _ := filepath.Rel(root, p)
			fmt.Fprintf(&b, "\n/* ---- %s ---- */\n", filepath.ToSlash(rel))
			b.WriteString(readNorm(p))
		}
		return b.String()
	}
	checks := map[string]string{
		filepath.Join(root, "ui/js/app.bundle.js"):         buildJS("app"),
		filepath.Join(root, "ui/js/app.control.bundle.js"): buildJS("control"),
		filepath.Join(root, "ui/dashboard.css"):            buildCSS("dashboard", "dashboard.css"),
		filepath.Join(root, "ui/control-layout.css"):       buildCSS("control", "control-layout.css"),
	}
	for p, exp := range checks {
		if write {
			if err := os.WriteFile(p, []byte(exp), 0644); err != nil {
				return err
			}
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("missing generated asset: %s", p)
		}
		if string(b) != exp {
			rel, _ := filepath.Rel(root, p)
			return fmt.Errorf("stale generated asset: %s", filepath.ToSlash(rel))
		}
	}
	// The generated bundles are versioned in index.html. Matching source and
	// bundle bytes is not sufficient when the page still requests an older query
	// string, so make the cache-buster part of the same release-blocking check.
	index := readNorm(filepath.Join(root, "index.html"))
	for _, want := range []string{
		"ui/dashboard.css?v=" + version,
		"ui/js/app.bundle.js?v=" + version,
	} {
		if !strings.Contains(index, want) {
			return fmt.Errorf("stale asset cache-buster in index.html: expected %s", want)
		}
	}
	// Dashboard Control is intentionally lazy-loaded. Its generated stylesheet
	// must not return to the boot document, and its runtime CONFIG.version must
	// remain the release cache-buster for both Control assets.
	if strings.Contains(index, `href="ui/control-layout.css`) {
		return fmt.Errorf("control-layout.css must be lazy-loaded, not linked by index.html")
	}
	defaults, err := os.ReadFile(filepath.Join(root, "ui", "js", "config-defaults.js"))
	if err != nil {
		return fmt.Errorf("missing runtime defaults: %w", err)
	}
	if !strings.Contains(string(defaults), `version: "`+version+`"`) {
		return fmt.Errorf("stale lazy control cache-buster in ui/js/config-defaults.js: expected %s", version)
	}
	lazyLoader, err := os.ReadFile(filepath.Join(root, "ui", "js", "control-lazy-loader.js"))
	if err != nil {
		return fmt.Errorf("missing control lazy loader: %w", err)
	}
	if !strings.Contains(string(lazyLoader), `controlAssetURL("ui/control-layout.css")`) || !strings.Contains(string(lazyLoader), `controlAssetURL("ui/js/app.control.bundle.js")`) {
		return fmt.Errorf("control lazy loader must version both Control CSS and JavaScript")
	}
	return nil
}

func (a *app) runPackageScrubCLI(args []string) int {
	root := a.dash
	if len(args) > 0 && args[0] != "" {
		root = args[0]
	}
	errs := packageScrubErrors(root)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "package scrub failed:", e)
		}
		return 1
	}
	fmt.Println("package scrub check passed")
	return 0
}

func packageScrubErrors(root string) []string {
	errs := []string{}
	read := func(rel string) (string, bool) {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return "", false
		}
		return string(b), true
	}
	for _, rel := range []string{"cache/action-history.json", "logs/map-prewarm.log"} {
		txt, ok := read(rel)
		if !ok {
			continue
		}
		if strings.HasSuffix(rel, ".json") {
			var v any
			if json.Unmarshal([]byte(txt), &v) != nil {
				errs = append(errs, rel+" is present and is not valid empty JSON")
			} else if len(jsonutil.List(v)) > 0 || len(jsonutil.Map(v)) > 0 {
				errs = append(errs, rel+" contains runtime entries")
			}
		} else if strings.TrimSpace(txt) != "" {
			errs = append(errs, rel+" contains runtime log output")
		}
	}
	if txt, ok := read("cache/events.cache.json"); ok && strings.TrimSpace(txt) != "" {
		m := jsonutil.Map(jsonFromString(txt))
		if len(jsonutil.List(m["events"])) > 0 || len(jsonutil.List(m["issues"])) > 0 {
			errs = append(errs, "cache/events.cache.json contains generated runtime data")
		}
	}
	if txt, ok := read("config/config.local.js"); ok {
		compact := runtimeAssetWhitespace.ReplaceAllString(txt, "")
		if runtimeAssetConfigLatitude.MatchString(txt) || runtimeAssetConfigLongitude.MatchString(txt) {
			errs = append(errs, "config/config.local.js contains lat/lon runtime coordinates")
		}
		if strings.Contains(txt, "locationName:") || strings.Contains(compact, "demoMode:true") {
			errs = append(errs, "config/config.local.js contains location/demo runtime state")
		}
	}
	return errs
}

func jsonFromString(s string) any { var v any; _ = json.Unmarshal([]byte(s), &v); return v }
