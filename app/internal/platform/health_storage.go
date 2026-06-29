package platform

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func BootEpochMillis(now time.Time) int64 {
	body, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return 0
	}
	seconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return now.Add(-time.Duration(seconds * float64(time.Second))).UnixMilli()
}
func HealthStateEpochMillis(raw map[string]any, key string) int64 {
	value := int64(jsonutil.Int(raw[key], 0))
	if value > 0 && value < 100000000000 {
		value *= 1000
	}
	return value
}
func StorageKernelWarningPredatesBoot(raw map[string]any, fact HealthFact, bootMillis int64) bool {
	if raw == nil || fact.Level != "degraded" || bootMillis <= 0 {
		return false
	}
	if raw["readOnly"] == true || !strings.EqualFold(strings.TrimSpace(jsonutil.TextValue(raw["canary"])), "ok") {
		return false
	}
	if jsonutil.Int(raw["kernelErrorsCurrentBoot"], 0) < 3 {
		return false
	}
	if freeKB := jsonutil.Int(raw["freeKB"], 0); freeKB > 0 && freeKB < 512*1024 {
		return false
	}
	updated := HealthStateEpochMillis(raw, "updated")
	return updated > 0 && updated < bootMillis
}
