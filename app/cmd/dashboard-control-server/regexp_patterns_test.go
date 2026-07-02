package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// These names were removed from the command package after their behavior moved
// into focused internal packages. Keep the startup regexp inventory small: a
// future caller must use the owning helper instead of reviving a process-wide
// pattern that every server and short-lived CLI invocation compiles.
var removedStartupRegexIdentifiers = []string{
	"reDefaultCalendarKey", "reICSUIDSafe", "reHolidayDate", "reISSLatitude", "reISSLongitude",
	"reHexSix", "reHexColor", "reMonthDay", "reISODate", "reANSI", "reDoctorLine", "reAbsoluteURL",
	"reMapCacheExtension", "reMapCacheCoordinate", "reWhitespace", "reControlChars", "reUSPostal",
	"reUSCountry", "reStreetNumber", "reStreetSuffix", "reUSCountrySuffix", "reTimeOfDay",
	"reBirthdayList", "reBirthdayListLine", "reFeedExpiry", "reNSFWPrefix", "reConfigLatitude",
	"reConfigLongitude", "reLocationName", "reSkyLatitude", "reSkyLongitude", "reFirstDecimal",
	"reJSStringArrayItem", "reJSStringMapItem", "reWeatherSnow", "reWeatherRain", "reWeatherFog",
	"reFirstNumber",
}

func TestRemovedStartupRegexesHaveNoRemainingCommandReferences(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate command source")
	}
	root := filepath.Dir(thisFile)
	for _, identifier := range removedStartupRegexIdentifiers {
		matcher := regexp.MustCompile(`\b` + regexp.QuoteMeta(identifier) + `\b`)
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || filepath.Base(path) == "regexp_patterns_test.go" {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if matcher.Match(body) {
				t.Fatalf("removed startup regexp %s is still referenced by %s", identifier, strings.TrimPrefix(path, root+string(filepath.Separator)))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
