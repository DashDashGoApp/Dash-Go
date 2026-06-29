package main

import (
	"testing"
	"time"
)

func TestStorageKernelWarningPredatesBootOnlySuppressesStaleKernelEvidence(t *testing.T) {
	now := time.Date(2030, 4, 5, 6, 7, 8, 0, time.UTC)
	boot := now.Add(-20 * time.Minute).UnixMilli()
	base := map[string]any{
		"level": "warn", "reason": "current boot kernel log contains storage I/O or filesystem errors",
		"updated": now.Add(-2 * time.Hour).Unix(), "readOnly": false, "canary": "ok",
		"freeKB": 1024 * 1024, "kernelErrorsCurrentBoot": 3,
	}
	fact := healthFact{Name: "storage", Tier: "device", Level: "degraded", Reason: "current boot kernel log contains storage I/O or filesystem errors"}
	if !storageKernelWarningPredatesBoot(base, fact, boot) {
		t.Fatalf("prior-boot kernel evidence should be stale: raw=%#v fact=%#v", base, fact)
	}
	fresh := map[string]any{}
	for k, v := range base {
		fresh[k] = v
	}
	fresh["updated"] = now.Add(-2 * time.Minute).Unix()
	if storageKernelWarningPredatesBoot(fresh, fact, boot) {
		t.Fatalf("current-boot kernel evidence was hidden: raw=%#v", fresh)
	}
	for name, mutate := range map[string]func(map[string]any){
		"read-only mount": func(raw map[string]any) { raw["readOnly"] = true },
		"canary failure":  func(raw map[string]any) { raw["canary"] = "failed" },
		"low free space":  func(raw map[string]any) { raw["freeKB"] = 100 },
		"no kernel proof": func(raw map[string]any) { raw["kernelErrorsCurrentBoot"] = 0 },
	} {
		raw := map[string]any{}
		for k, v := range base {
			raw[k] = v
		}
		mutate(raw)
		if storageKernelWarningPredatesBoot(raw, fact, boot) {
			t.Fatalf("%s must remain visible: raw=%#v", name, raw)
		}
	}
}
