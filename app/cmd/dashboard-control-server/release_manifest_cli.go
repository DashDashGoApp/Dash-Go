package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Release-manifest CLI helpers are deliberately isolated from update-record
// persistence and stale-source cleanup. The manifest is a downloaded trust
// boundary; keep its parsing and verification rules easy to audit.

type releaseManifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type releaseManifest struct {
	Version string                `json:"version"`
	Files   []releaseManifestFile `json:"files"`
}

// Release manifests are a downloaded trust boundary. Keep their parse window
// intentionally small: a normal Dash-Go manifest is only a few KiB, while a
// multi-megabyte JSON payload is never a legitimate release catalog.
const maxReleaseManifestBytes int64 = 4 << 20

func safeReleasePath(rel string) bool {
	rel = strings.TrimSpace(filepath.ToSlash(rel))
	if rel == "" || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "../") || rel == "." {
		return false
	}
	for part := range strings.SplitSeq(rel, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func readReleaseManifest(path string) (releaseManifest, error) {
	info, err := os.Stat(path)
	if err != nil {
		return releaseManifest{}, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxReleaseManifestBytes {
		return releaseManifest{}, errors.New("manifest size is invalid")
	}
	f, err := os.Open(path)
	if err != nil {
		return releaseManifest{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxReleaseManifestBytes+1))
	if err != nil {
		return releaseManifest{}, err
	}
	if int64(len(b)) > maxReleaseManifestBytes {
		return releaseManifest{}, errors.New("manifest is too large")
	}
	var manifest releaseManifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return releaseManifest{}, fmt.Errorf("manifest JSON error: %w", err)
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return releaseManifest{}, errors.New("manifest version is missing")
	}
	if len(manifest.Files) == 0 {
		return releaseManifest{}, errors.New("manifest has no files list")
	}
	seen := map[string]bool{}
	for _, item := range manifest.Files {
		if !safeReleasePath(item.Path) {
			return releaseManifest{}, fmt.Errorf("unsafe manifest path: %q", item.Path)
		}
		if seen[item.Path] {
			return releaseManifest{}, fmt.Errorf("duplicate manifest path: %s", item.Path)
		}
		seen[item.Path] = true
	}
	return manifest, nil
}

func hashFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.CopyBuffer(h, f, make([]byte, 1024*1024)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (a *app) runVerifyReleaseManifestCLI(args []string) int {
	fs := flag.NewFlagSet("verify-release-manifest", flag.ContinueOnError)
	manifestPath := fs.String("manifest", "", "manifest JSON path")
	root := fs.String("root", "", "release root")
	version := fs.String("version", "", "expected release version")
	targetBin := fs.String("target-bin", "", "host architecture server binary")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *manifestPath == "" || *root == "" || *version == "" {
		fmt.Fprintln(os.Stderr, "usage: --verify-release-manifest --manifest FILE --root DIR --version VERSION [--target-bin RELPATH]")
		return 64
	}
	manifest, err := readReleaseManifest(*manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if manifest.Version != *version {
		fmt.Fprintln(os.Stderr, "manifest version mismatch")
		return 1
	}
	for _, item := range manifest.Files {
		path := filepath.Join(*root, filepath.FromSlash(item.Path))
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			fmt.Fprintf(os.Stderr, "missing manifest file: %s\n", item.Path)
			return 1
		}
		crossBinary := strings.HasPrefix(item.Path, "bin/dashboard-control-server-linux-") && *targetBin != "" && item.Path != *targetBin
		if item.SHA256 == "" || item.Path == "manifest.json" || crossBinary {
			continue
		}
		actual, err := hashFileSHA256(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if !strings.EqualFold(actual, item.SHA256) {
			fmt.Fprintf(os.Stderr, "hash mismatch for %s\n", item.Path)
			return 1
		}
	}
	return 0
}

func releaseInstallFiles(manifest releaseManifest) []string {
	skipPrefixes := []string{"config/", "calendars/", "cache/", "logs/", "releases/", ".git/"}
	skipExact := map[string]bool{"install.sh": true, "AI.md": true}
	out := make([]string, 0, len(manifest.Files)+1)
	seen := map[string]bool{}
	for _, item := range manifest.Files {
		rel := item.Path
		skip := skipExact[rel]
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(rel, prefix) {
				skip = true
				break
			}
		}
		if !skip && !seen[rel] {
			seen[rel] = true
			out = append(out, rel)
		}
	}
	if !seen["manifest.json"] {
		out = append(out, "manifest.json")
	}
	slices.Sort(out)
	return out
}

func (a *app) runReleaseFileListCLI(args []string) int {
	fs := flag.NewFlagSet("release-file-list", flag.ContinueOnError)
	manifestPath := fs.String("manifest", "", "manifest JSON path")
	if err := fs.Parse(args); err != nil {
		return 64
	}
	if *manifestPath == "" {
		fmt.Fprintln(os.Stderr, "usage: --release-file-list --manifest FILE")
		return 64
	}
	manifest, err := readReleaseManifest(*manifestPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, rel := range releaseInstallFiles(manifest) {
		fmt.Println(rel)
	}
	return 0
}
