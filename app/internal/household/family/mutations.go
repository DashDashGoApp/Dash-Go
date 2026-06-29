package family

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func NewID(now time.Time) string { return fmt.Sprintf("fb_%d", now.In(time.Local).UnixNano()) }

func WholeNumber(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if math.Trunc(n) != n || n > float64(math.MaxInt) || n < float64(math.MinInt) {
			return 0, false
		}
		return int(n), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(n))
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func Expiration(note, body map[string]any, create bool, now time.Time) (string, error) {
	raw, provided := body["expiration"]
	if provided {
		expiration, ok := raw.(map[string]any)
		if !ok {
			return "", fmt.Errorf("expiration must be a valid choice")
		}
		switch strings.ToLower(String(expiration["kind"])) {
		case "none":
			return "", nil
		case "keep":
			if create {
				return "", fmt.Errorf("new family notes cannot keep an existing expiration")
			}
			return ExpiryStamp(note["expiresAt"]), nil
		case "date":
			date := Date(expiration["date"])
			if date == "" {
				return "", fmt.Errorf("expiration date must be YYYY-MM-DD")
			}
			return ExpiryAtEndOfLocalDate(date), nil
		case "duration":
			amount, ok := WholeNumber(expiration["amount"])
			if !ok || amount < 1 {
				return "", fmt.Errorf("expiration amount must be a whole number")
			}
			switch strings.ToLower(String(expiration["unit"])) {
			case "minutes":
				if amount > 1440 {
					return "", fmt.Errorf("minute expiration must be between 1 and 1440")
				}
				return now.Add(time.Duration(amount) * time.Minute).In(time.Local).Format(time.RFC3339), nil
			case "hours":
				if amount > 168 {
					return "", fmt.Errorf("hour expiration must be between 1 and 168")
				}
				return now.Add(time.Duration(amount) * time.Hour).In(time.Local).Format(time.RFC3339), nil
			default:
				return "", fmt.Errorf("expiration duration unit must be minutes or hours")
			}
		default:
			return "", fmt.Errorf("unknown expiration choice")
		}
	}
	if create {
		return "", nil
	}
	return ExpiryStamp(note["expiresAt"]), nil
}

func MutableNote(note, body map[string]any, create bool, now time.Time) error {
	rawText := String(body["text"])
	if rawText == "" {
		return fmt.Errorf("family note text is required")
	}
	if len([]rune(rawText)) > 320 {
		return fmt.Errorf("family note must be 320 characters or fewer")
	}
	expiresAt, err := Expiration(note, body, create, now)
	if err != nil {
		return err
	}
	note["scope"] = "household"
	note["text"] = Text(rawText, 320)
	note["priority"] = Priority(body["priority"])
	note["pinned"] = jsonutil.Truthy(body["pinned"])
	note["expiresAt"] = expiresAt
	note["householdAcknowledgedAt"] = ""
	note["updatedAt"] = ArchiveStamp(now)
	if create {
		note["id"] = NewID(now)
		note["state"] = "active"
		note["createdAt"] = note["updatedAt"]
		note["archivedAt"] = ""
	}
	return nil
}
