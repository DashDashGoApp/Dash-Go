package main

import (
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func TestDoctorReportParsesLegacyFailuresAndRepairs(t *testing.T) {
	report := (&app{}).parseDoctorReport(`== Installation
OK  selector works
BAD legacy doctor failure
== Configuration
FIXED repaired invalid settings
WARN location is missing
`, 1, 2*time.Second)

	if report["ok"] != false || jsonutil.Int(report["failCount"], 0) != 1 {
		t.Fatalf("legacy BAD line was not treated as a failure: %#v", report)
	}
	if jsonutil.Int(report["fixCount"], 0) != 1 || jsonutil.Int(report["passCount"], 0) != 2 {
		t.Fatalf("FIXED line was not counted as a repaired pass: %#v", report)
	}
	if jsonutil.Int(report["warnCount"], 0) != 1 || jsonutil.Int(report["issueCount"], 0) != 2 {
		t.Fatalf("warning/issue counts are incorrect: %#v", report)
	}
	issues := report["issues"].([]map[string]any)
	if len(issues) != 2 || issues[0]["level"] != "fail" || issues[0]["section"] != "Installation" {
		t.Fatalf("legacy failure was not exposed to Dashboard Control: %#v", issues)
	}
}

func TestDoctorReportAddsFailureForUnstructuredExit(t *testing.T) {
	report := (&app{}).parseDoctorReport("doctor crashed before reporting status\n", 7, time.Second)
	if report["ok"] != false || jsonutil.Int(report["failCount"], 0) != 1 {
		t.Fatalf("nonzero unstructured doctor exit was hidden: %#v", report)
	}
	issues := report["issues"].([]map[string]any)
	if len(issues) != 1 || issues[0]["section"] != "Doctor" {
		t.Fatalf("doctor exit issue missing: %#v", issues)
	}
}
