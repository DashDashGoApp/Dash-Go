package todo

import (
	"fmt"
	"unicode/utf8"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Request-size bounds protect the local task cache and persisted pending-write
// queue. They are intentionally compatible with Microsoft opaque IDs and the
// normal touch UI while stopping values that would otherwise be retained only
// to be truncated or retried later.
const (
	maxTodoTaskIDRunes      = 512
	maxTodoTaskTitleRunes   = 255
	maxTodoTaskStatusRunes  = 32
	maxTodoImportanceRunes  = 32
	maxTodoListNameRunes    = 255
	maxTodoClientIDRunes    = 256
	maxTodoClearSnapshotIDs = 512
)

func validateTodoString(value, label string, limit int) error {
	if utf8.RuneCountInString(value) > limit {
		return fmt.Errorf("%s must be %d characters or fewer", label, limit)
	}
	return nil
}

func validateTodoOptionalString(body map[string]any, key, label string, limit int) error {
	value, ok := body[key].(string)
	if !ok {
		return nil
	}
	return validateTodoString(value, label, limit)
}

func validateTodoTaskBody(body map[string]any) error {
	if err := validateTodoOptionalString(body, "id", "task id", maxTodoTaskIDRunes); err != nil {
		return err
	}
	if err := validateTodoOptionalString(body, "title", "task title", maxTodoTaskTitleRunes); err != nil {
		return err
	}
	if err := validateTodoOptionalString(body, "status", "task status", maxTodoTaskStatusRunes); err != nil {
		return err
	}
	return validateTodoOptionalString(body, "importance", "task importance", maxTodoImportanceRunes)
}

func validateTodoTaskIDs(raw any) error {
	values, ok := raw.([]any)
	if !ok {
		return nil
	}
	if len(values) > maxTodoClearSnapshotIDs {
		return fmt.Errorf("completed-task snapshot is limited to %d task ids", maxTodoClearSnapshotIDs)
	}
	for _, rawID := range values {
		if err := validateTodoString(jsonutil.StringValue(rawID), "task id", maxTodoTaskIDRunes); err != nil {
			return err
		}
	}
	return nil
}

// ValidateTaskBody is used by the HTTP adapter before a local write can enter
// the durable task cache or a pending Microsoft Graph operation.
func ValidateTaskBody(body map[string]any) error { return validateTodoTaskBody(body) }

// ValidateTaskIDs checks the explicit completion snapshot accepted by the
// destructive clear-completed action. Omitting it retains the established
// server-side snapshot behavior.
func ValidateTaskIDs(raw any) error { return validateTodoTaskIDs(raw) }

func ValidateListID(value string) error {
	return validateTodoString(value, "list id", maxTodoTaskIDRunes)
}

func ValidateTaskID(value string) error {
	return validateTodoString(value, "task id", maxTodoTaskIDRunes)
}

func ValidateListDisplayName(value string) error {
	return validateTodoString(value, "list name", maxTodoListNameRunes)
}

func ValidateClientID(value string) error {
	return validateTodoString(value, "client id", maxTodoClientIDRunes)
}
