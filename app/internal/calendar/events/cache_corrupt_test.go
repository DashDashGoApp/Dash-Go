package events

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheMetadataFromExistingLogsUnparseableSources(t *testing.T) {
	buf := &bytes.Buffer{}
	prev := log.Writer()
	log.SetOutput(buf)
	defer log.SetOutput(prev)

	anchor := filepath.Join(t.TempDir(), "anchor")
	if err := os.WriteFile(anchor, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(anchor)
	if err != nil {
		t.Fatal(err)
	}
	oldCache := map[string]any{
		"version":     CacheVersion,
		"sources":     "not-an-array",
		"windowStart": float64(1000),
		"windowEnd":   float64(2000),
		"events":      []any{},
	}
	meta := cacheMetadataFromExisting("fp-1", cacheMetadata{}, oldCache, info, 1234)
	if meta.Sources != nil {
		t.Fatalf("unparseable sources must fall back to empty metadata: %#v", meta.Sources)
	}
	if meta.Fingerprint != "fp-1" || meta.LastSuccessAt != 1234 {
		t.Fatalf("unrelated metadata fields changed: %#v", meta)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "sources") {
		t.Fatalf("expected exactly one diagnostic, got %q", buf.String())
	}
}
