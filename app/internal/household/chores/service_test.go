package chores

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

func testService(t *testing.T) *Service {
	t.Helper()
	return New(ServiceConfig{ConfigDir: t.TempDir(), Now: func() time.Time { return time.Date(2026, 6, 24, 12, 0, 0, 0, time.Local) }})
}

func TestNormalizePreservesPayloadShapeAndCanonicalAssignments(t *testing.T) {
	payload := NormalizeAt(map[string]any{
		"revision":    3,
		"people":      []any{map[string]any{"id": "sam", "name": "Sam"}, map[string]any{"id": "sam", "name": "Duplicate"}},
		"chores":      []any{map[string]any{"id": "dishes", "name": "Dishes", "cadence": map[string]any{"type": "days", "every": 2, "anchorDate": "2026-06-20"}, "eligible": []any{"sam"}}},
		"assignments": []any{map[string]any{"id": "later", "date": "2026-06-25", "choreId": "dishes", "personId": "sam", "status": "assigned"}, map[string]any{"id": "earlier", "date": "2026-06-24", "choreId": "dishes", "personId": "sam", "status": "completed"}},
	}, time.Date(2026, 6, 24, 12, 0, 0, 0, time.Local))
	if got := jsonutil.Int(payload["revision"], -1); got != 3 {
		t.Fatalf("revision=%d", got)
	}
	if got := len(jsonutil.List(payload["people"])); got != 1 {
		t.Fatalf("people=%#v", payload["people"])
	}
	assignments := jsonutil.List(payload["assignments"])
	if got := DateKey(jsonutil.Map(assignments[0])["date"]); got != "2026-06-24" {
		t.Fatalf("assignment sort=%#v", assignments)
	}
}

func TestFairPlannerUsesStableLoadAndRecentTieBreaks(t *testing.T) {
	s := testService(t)
	payload := NormalizeAt(map[string]any{
		"people":      []any{map[string]any{"id": "alex", "name": "Alex"}, map[string]any{"id": "sam", "name": "Sam"}},
		"chores":      []any{map[string]any{"id": "dishes", "name": "Dishes", "effort": 2, "eligible": []any{"alex", "sam"}}},
		"assignments": []any{map[string]any{"id": "old-alex", "date": "2026-06-23", "choreId": "dishes", "personId": "alex", "status": "assigned"}},
	}, s.Now())
	chore := jsonutil.Map(jsonutil.List(payload["chores"])[0])
	winner := FairCandidate(payload, chore, "2026-06-24", "")
	if got := ID(winner["id"]); got != "sam" {
		t.Fatalf("winner=%#v, want Sam after Alex's earlier load", winner)
	}
	planned := s.GenerateDueAssignments(payload, "2026-06-24", "batch")
	if len(planned) != 1 || ID(jsonutil.Map(planned[0])["personId"]) != "sam" {
		t.Fatalf("planned=%#v", planned)
	}
}

func TestDayAndCompletionPreserveFutureGuard(t *testing.T) {
	s := testService(t)
	payload := NormalizeAt(map[string]any{
		"people":      []any{map[string]any{"id": "sam", "name": "Sam"}},
		"chores":      []any{map[string]any{"id": "dishes", "name": "Dishes"}},
		"assignments": []any{map[string]any{"id": "today", "date": "2026-06-24", "choreId": "dishes", "personId": "sam", "status": "assigned"}, map[string]any{"id": "future", "date": "2026-06-25", "choreId": "dishes", "personId": "sam", "status": "assigned"}},
	}, s.Now())
	day := s.DayResponse(payload, "2026-06-24")
	if len(jsonutil.List(day["items"])) != 1 || !jsonutil.Truthy(jsonutil.Map(jsonutil.List(day["items"])[0])["actionable"]) {
		t.Fatalf("day=%#v", day)
	}
	next, changed, err := s.CompleteAssignment(payload, "today", "2026-06-24")
	if err != nil || !changed || Text(jsonutil.Map(jsonutil.List(next["assignments"])[0])["status"], 16) != "completed" {
		t.Fatalf("completion next=%#v changed=%v err=%v", next, changed, err)
	}
	if _, _, err := s.CompleteAssignment(payload, "future", "2026-06-25"); err != ErrFutureComplete {
		t.Fatalf("future completion err=%v", err)
	}
}

func TestCalendarProjectionAndPeopleReconciliationKeepHistory(t *testing.T) {
	s := testService(t)
	payload := NormalizeAt(map[string]any{
		"people":      []any{map[string]any{"id": "sam", "name": "Sam"}, map[string]any{"id": "alex", "name": "Alex"}},
		"chores":      []any{map[string]any{"id": "dishes", "name": "Dishes", "eligible": []any{"sam", "alex"}}},
		"assignments": []any{map[string]any{"id": "past", "date": "2026-06-23", "choreId": "dishes", "personId": "sam", "status": "completed"}, map[string]any{"id": "today", "date": "2026-06-24", "choreId": "dishes", "personId": "sam", "status": "assigned"}, map[string]any{"id": "future", "date": "2026-06-25", "choreId": "dishes", "personId": "sam", "status": "assigned"}},
	}, s.Now())
	next, err := s.ReconcilePeople(payload, []any{map[string]any{"id": "alex", "name": "Alex"}}, "delete", "sam", "alex", "Alex")
	if err != nil {
		t.Fatal(err)
	}
	assignments := jsonutil.List(next["assignments"])
	if got := ID(jsonutil.Map(assignments[0])["personId"]); got != "sam" {
		t.Fatalf("historical snapshot changed=%#v", assignments)
	}
	if got := ID(jsonutil.Map(assignments[1])["personId"]); got != "alex" {
		t.Fatalf("current assignment was not reassigned=%#v", assignments)
	}
	events := s.CalendarEvents(next)
	if len(events) != 2 || events[0].AppOwner != "chore-wheel" {
		t.Fatalf("events=%#v", events)
	}
}

func TestServiceWritesCanonicalJSON(t *testing.T) {
	s := testService(t)
	if err := s.Write(map[string]any{"people": []any{map[string]any{"id": "sam", "name": "Sam"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(s.File()), "chore-wheel.json")); err != nil {
		t.Fatal(err)
	}
	if got := ID(jsonutil.Map(jsonutil.List(s.Payload()["people"])[0])["id"]); got != "sam" {
		t.Fatalf("persisted payload=%#v", s.Payload())
	}
}

func TestCalendarCheckboxCanReopenCurrentCompletion(t *testing.T) {
	s := testService(t)
	payload := NormalizeAt(map[string]any{
		"people":      []any{map[string]any{"id": "sam", "name": "Sam"}},
		"chores":      []any{map[string]any{"id": "dishes", "name": "Dishes"}},
		"assignments": []any{map[string]any{"id": "today", "date": "2026-06-24", "choreId": "dishes", "personId": "sam", "status": "completed"}},
	}, s.Now())
	day := s.DayResponse(payload, "2026-06-24")
	item := jsonutil.Map(jsonutil.List(day["items"])[0])
	if !jsonutil.Truthy(item["actionable"]) {
		t.Fatalf("completed current assignment must be reversible: %#v", day)
	}
	next, changed, err := s.SetAssignmentCompleted(payload, "today", "2026-06-24", false)
	if err != nil || !changed {
		t.Fatalf("reopen changed=%v err=%v", changed, err)
	}
	assignment := jsonutil.Map(jsonutil.List(next["assignments"])[0])
	if got := Text(assignment["status"], 16); got != "assigned" {
		t.Fatalf("status=%q want assigned", got)
	}
}

func TestCalendarCheckboxCannotReopenSkippedAssignment(t *testing.T) {
	s := testService(t)
	payload := NormalizeAt(map[string]any{
		"people":      []any{map[string]any{"id": "sam", "name": "Sam"}},
		"chores":      []any{map[string]any{"id": "dishes", "name": "Dishes"}},
		"assignments": []any{map[string]any{"id": "today", "date": "2026-06-24", "choreId": "dishes", "personId": "sam", "status": "skipped"}},
	}, s.Now())
	if _, _, err := s.SetAssignmentCompleted(payload, "today", "2026-06-24", false); err != ErrAssignmentStatus {
		t.Fatalf("skipped reopen err=%v", err)
	}
}

func TestEveryNDaysCadenceUsesCivilDatesAcrossSpringDST(t *testing.T) {
	original := time.Local
	location, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	time.Local = location
	defer func() { time.Local = original }()

	now := time.Date(2026, time.March, 1, 12, 0, 0, 0, location)
	weekly := map[string]any{"cadence": map[string]any{"type": "days", "every": 7, "anchorDate": "2026-03-01"}}
	if !Due(weekly, "2026-03-15", now) {
		t.Fatal("every-7-days chore was not due one civil week after the DST transition")
	}
	if Due(weekly, "2026-03-14", now) {
		t.Fatal("every-7-days chore became due on the wrong civil day after DST")
	}
	alternate := map[string]any{"cadence": map[string]any{"type": "days", "every": 2, "anchorDate": "2026-03-01"}}
	if !Due(alternate, "2026-03-09", now) || Due(alternate, "2026-03-10", now) {
		t.Fatal("every-2-days cadence did not preserve its civil-day parity across DST")
	}
}
