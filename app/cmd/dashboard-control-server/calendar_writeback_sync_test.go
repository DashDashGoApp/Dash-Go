package main

import (
	"slices"
	"strings"
	"testing"
)

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

func TestWritebackSyncArgsArmsEmptyOverrideOnlyForVerifiedFinalDelete(t *testing.T) {
	regular := writebackSyncArgs("dash_family", false)
	if slices.Contains(regular, "--allow-empty-once") {
		t.Fatalf("non-final delete args unexpectedly armed the empty override: %#v", regular)
	}
	final := writebackSyncArgs("dash_family", true)
	if !slices.Contains(final, "--allow-empty-once") {
		t.Fatalf("final delete args omitted the one-run empty override: %#v", final)
	}
	if got := writebackSyncArgs("__legacy__", true); len(got) != 0 {
		t.Fatalf("legacy source must not guess a targeted destructive pair: %#v", got)
	}
}

func TestWritebackSyncOutcomeClassifiesZeroExitSkipped(t *testing.T) {
	state, detail := writebackSyncOutcome("skipped", false, false, false)
	if state != "attention" {
		t.Fatalf("zero-exit skipped state = %q, want attention", state)
	}
	if strings.Contains(strings.ToLower(detail), "synchronized") {
		t.Fatalf("zero-exit skipped result was reported as synchronized: %q", detail)
	}
	if !strings.Contains(strings.ToLower(detail), "authorization") {
		t.Fatalf("zero-exit skipped result did not explain authorization: %q", detail)
	}
}

func TestWritebackSyncOutcomeKeepsFinalDeleteFailureHonest(t *testing.T) {
	state, detail := writebackSyncOutcome("", true, false, true)
	if state != "attention" {
		t.Fatalf("failed final delete state = %q, want attention", state)
	}
	if !strings.Contains(strings.ToLower(detail), "will not repeat") {
		t.Fatalf("failed final delete promised an automatic retry: %q", detail)
	}
}

func TestWritebackSyncOutcomeDoesNotTreatUnknownZeroExitResultAsSynced(t *testing.T) {
	state, detail := writebackSyncOutcome("unexpected-state", false, false, false)
	if state != "attention" {
		t.Fatalf("unknown zero-exit result state = %q, want attention", state)
	}
	if strings.Contains(strings.ToLower(detail), "synchronized") {
		t.Fatalf("unknown zero-exit result was reported as synchronized: %q", detail)
	}
	if !strings.Contains(strings.ToLower(detail), "unrecognized") {
		t.Fatalf("unknown zero-exit result did not explain its safe classification: %q", detail)
	}
}
