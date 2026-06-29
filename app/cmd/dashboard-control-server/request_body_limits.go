package main

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

// These limits apply to every JSON POST before route dispatch. They are kept
// deliberately above normal Dashboard Control, app, and Chalkboard payloads;
// domain owners retain their tighter semantic limits for names, notes, tasks,
// and messages.
const (
	maxJSONRequestObjectFields = 128
	maxJSONRequestArrayItems   = 2048
	maxJSONRequestDepth        = 16
	maxJSONRequestKeyRunes     = 96
	maxJSONRequestStringRunes  = 4096

	// The browser already limits the Chalkboard to 300 strokes. Keep the
	// server-side persisted shape equally bounded without changing a normal
	// saved board or introducing a second body-size allowance.
	maxChalkboardStrokes           = 300
	maxChalkboardStrokeCoordinates = 1024
	maxChalkboardFillSpans         = 1024
	maxChalkboardBoardRunes        = 24
)

var (
	errRequestBodyTooLarge = errors.New("request body too large")
	errRequestFieldLimit   = errors.New("request fields exceed supported limits")
)

func validateJSONRequestFields(body map[string]any) error {
	return validateJSONRequestValue(body, 0)
}

func validateJSONRequestValue(value any, depth int) error {
	if depth > maxJSONRequestDepth {
		return fmt.Errorf("%w: body nesting is too deep", errRequestFieldLimit)
	}
	switch typed := value.(type) {
	case string:
		if utf8.RuneCountInString(typed) > maxJSONRequestStringRunes {
			return fmt.Errorf("%w: a text field is too long", errRequestFieldLimit)
		}
	case map[string]any:
		if len(typed) > maxJSONRequestObjectFields {
			return fmt.Errorf("%w: an object has too many fields", errRequestFieldLimit)
		}
		for key, child := range typed {
			if utf8.RuneCountInString(key) > maxJSONRequestKeyRunes {
				return fmt.Errorf("%w: a field name is too long", errRequestFieldLimit)
			}
			if err := validateJSONRequestValue(child, depth+1); err != nil {
				return err
			}
		}
	case []any:
		if len(typed) > maxJSONRequestArrayItems {
			return fmt.Errorf("%w: a list has too many items", errRequestFieldLimit)
		}
		for _, child := range typed {
			if err := validateJSONRequestValue(child, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateChalkboardPayload keeps the persisted board within the already
// shipped client-side limits. The generic body cap still remains the single
// transport limit for every endpoint, including Chalkboard.
func validateChalkboardPayload(body map[string]any) error {
	if board, ok := body["board"].(string); ok && utf8.RuneCountInString(board) > maxChalkboardBoardRunes {
		return fmt.Errorf("%w: chalkboard board name is too long", errRequestFieldLimit)
	}
	rawStrokes, exists := body["strokes"]
	if !exists {
		// Preserve the historical permissive empty-board write shape used by
		// older clients; any present strokes field must be a bounded list.
		return nil
	}
	strokes, ok := rawStrokes.([]any)
	if !ok {
		return fmt.Errorf("%w: chalkboard strokes must be a list", errRequestFieldLimit)
	}
	if len(strokes) > maxChalkboardStrokes {
		return fmt.Errorf("%w: chalkboard has too many strokes", errRequestFieldLimit)
	}
	for _, rawStroke := range strokes {
		stroke, ok := rawStroke.(map[string]any)
		if !ok {
			return fmt.Errorf("%w: chalkboard stroke is invalid", errRequestFieldLimit)
		}
		if err := validateChalkboardStrokeList(stroke, "pts", maxChalkboardStrokeCoordinates); err != nil {
			return err
		}
		if err := validateChalkboardStrokeList(stroke, "spans", maxChalkboardFillSpans); err != nil {
			return err
		}
	}
	return nil
}

func validateChalkboardStrokeList(stroke map[string]any, key string, limit int) error {
	raw, exists := stroke[key]
	if !exists {
		return nil
	}
	values, ok := raw.([]any)
	if !ok || len(values) > limit {
		return fmt.Errorf("%w: chalkboard %s exceeds its limit", errRequestFieldLimit, key)
	}
	return nil
}
