// Package jsonutil provides small, dependency-free helpers for decoded JSON-like values.
package jsonutil

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
)

// StringValue accepts only strings from a fixed schema. It is the default for
// HTTP bodies and provider fields so numbers, objects, and arrays cannot become
// accidental identifiers or user-visible labels through fmt.Sprint.
func StringValue(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// TextValue converts tolerated decoded input to trimmed text.
// Use it only for diagnostic, cache, report, and compatibility-tolerant paths
// where preserving established scalar stringification is meaningful.
// Schema-defined fields must use StringValue.
// It is nil-safe, so a missing value does not synthesize fmt.Sprint's "<nil>" text.
func TextValue(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// BodyString reads one schema-defined string field from a JSON request body.
func BodyString(body map[string]any, key string) string {
	return StringValue(body[key])
}

// Truthy recognizes the supported JSON-like truthy forms.
func Truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "1" || strings.EqualFold(x, "true")
	}
	return false
}

// Int returns an integer from the supported decoded scalar forms, or def.
func Int(v any, def int) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err == nil {
			return n
		}
	}
	return def
}

// List returns a decoded JSON list, or a non-nil empty list.
func List(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return []any{}
}

// Map returns a decoded JSON object, or a non-nil empty object.
func Map(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// CloneMap returns a shallow copy of m. A nil input produces a non-nil empty map.
func CloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	maps.Copy(out, m)
	return out
}
