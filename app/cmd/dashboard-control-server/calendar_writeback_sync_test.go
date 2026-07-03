package main

import "testing"

func TestWritebackPairResultUsesOnlyTheRequestedPair(t *testing.T) {
	output := "RESULT\tdash_other\tconflict\nRESULT\tdash_family\tsynced\n"
	if got := writebackPairResult(output, "dash_family"); got != "synced" {
		t.Fatalf("result = %q, want synced", got)
	}
}

func TestWritebackPairResultLegacyFallbackPrefersMostSeriousOutcome(t *testing.T) {
	output := "RESULT\tdash_first\tsynced\nRESULT\tdash_second\tattention-empty\nRESULT\tdash_third\tconflict\n"
	if got := writebackPairResult(output, "__legacy__"); got != "conflict" {
		t.Fatalf("legacy result = %q, want conflict", got)
	}
}

func TestWritebackPairResultRejectsMalformedRows(t *testing.T) {
	output := "RESULT dash_family conflict\nRESULT\tdash_family\nRESULT\tdash_family\tsynced\textra\n"
	if got := writebackPairResult(output, "dash_family"); got != "" {
		t.Fatalf("malformed result = %q, want empty", got)
	}
}
