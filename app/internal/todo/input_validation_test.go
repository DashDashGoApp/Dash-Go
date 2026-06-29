package todo

import (
	"strings"
	"testing"
)

func TestRequestFieldValidationRejectsOversizedTaskValues(t *testing.T) {
	if err := ValidateTaskBody(map[string]any{"title": strings.Repeat("x", maxTodoTaskTitleRunes+1)}); err == nil {
		t.Fatal("expected oversized task title to be rejected")
	}
	if err := ValidateTaskID(strings.Repeat("x", maxTodoTaskIDRunes+1)); err == nil {
		t.Fatal("expected oversized task id to be rejected")
	}
	ids := make([]any, maxTodoClearSnapshotIDs+1)
	for i := range ids {
		ids[i] = "task"
	}
	if err := ValidateTaskIDs(ids); err == nil {
		t.Fatal("expected oversized completed-task snapshot to be rejected")
	}
}

func TestRequestFieldValidationAllowsNormalTodoValues(t *testing.T) {
	if err := ValidateTaskBody(map[string]any{"id": "task-1", "title": "Milk", "status": "notStarted", "importance": "normal"}); err != nil {
		t.Fatalf("normal task values rejected: %v", err)
	}
	if err := ValidateListDisplayName("Family errands"); err != nil {
		t.Fatalf("normal list name rejected: %v", err)
	}
}
