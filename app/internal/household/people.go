package household

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// Default returns a new empty roster payload.
func Default() map[string]any {
	return map[string]any{"schema": Schema, "revision": 0, "people": []any{}}
}

// Text trims a decoded string and enforces the established rune limit.
func Text(value any, limit int) string {
	text, _ := value.(string)
	text = strings.TrimSpace(text)
	if len([]rune(text)) > limit {
		text = string([]rune(text)[:limit])
	}
	return text
}

// ID is the durable person-ID normalization shared by People and dependent
// household apps. It intentionally preserves the historical simple format.
func ID(value any) string { return strings.ReplaceAll(Text(value, 96), " ", "-") }

func stamp(value any) string {
	text := jsonutil.StringValue(value)
	if _, err := time.Parse(time.RFC3339, text); err != nil {
		return ""
	}
	return text
}

func state(value any) string {
	if strings.EqualFold(jsonutil.StringValue(value), "archived") {
		return "archived"
	}
	return "active"
}

// NormalizePerson preserves the existing roster shape and default timestamps.
func NormalizePerson(raw any, now time.Time) map[string]any {
	row := jsonutil.Map(raw)
	id, name := ID(row["id"]), Text(row["name"], 64)
	if id == "" || name == "" {
		return nil
	}
	created := stamp(row["createdAt"])
	if created == "" {
		created = now.Format(time.RFC3339)
	}
	updated := stamp(row["updatedAt"])
	if updated == "" {
		updated = created
	}
	return map[string]any{
		"id": id, "name": name, "state": state(row["state"]),
		"createdAt": created, "updatedAt": updated, "archivedAt": stamp(row["archivedAt"]),
	}
}

// Normalize validates, de-duplicates, and name-sorts one roster payload.
func Normalize(raw map[string]any, now time.Time) map[string]any {
	out := Default()
	out["revision"] = max(0, jsonutil.Int(raw["revision"], 0))
	seen := map[string]bool{}
	people := []any{}
	for _, item := range jsonutil.List(raw["people"]) {
		person := NormalizePerson(item, now)
		if person == nil {
			continue
		}
		id := ID(person["id"])
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		people = append(people, person)
		if len(people) >= MaxPeople {
			break
		}
	}
	slices.SortStableFunc(people, func(left, right any) int {
		return strings.Compare(Text(jsonutil.Map(left)["name"], 64), Text(jsonutil.Map(right)["name"], 64))
	})
	out["people"] = people
	return out
}

// Equal compares normalized payloads so harmless input ordering/shape noise
// does not create a roster revision or write.
func Equal(left, right map[string]any, now time.Time) bool {
	lb, lerr := json.Marshal(Normalize(left, now))
	rb, rerr := json.Marshal(Normalize(right, now))
	return lerr == nil && rerr == nil && bytes.Equal(lb, rb)
}

// Active returns currently assignable people only.
func Active(payload map[string]any) []any {
	out := []any{}
	for _, raw := range jsonutil.List(payload["people"]) {
		person := jsonutil.Map(raw)
		if person["state"] == "active" {
			out = append(out, person)
		}
	}
	return out
}

// Merge seeds an empty canonical roster from legacy app-local candidate lists.
func Merge(base map[string]any, now time.Time, candidates ...[]any) map[string]any {
	out := Normalize(base, now)
	byID := map[string]map[string]any{}
	for _, raw := range jsonutil.List(out["people"]) {
		person := jsonutil.Map(raw)
		byID[ID(person["id"])] = person
	}
	for _, list := range candidates {
		for _, raw := range list {
			person := NormalizePerson(raw, now)
			if person == nil {
				continue
			}
			id := ID(person["id"])
			if byID[id] != nil {
				continue
			}
			byID[id] = person
		}
	}
	people := []any{}
	for _, person := range byID {
		people = append(people, person)
	}
	slices.SortStableFunc(people, func(left, right any) int {
		return strings.Compare(Text(jsonutil.Map(left)["name"], 64), Text(jsonutil.Map(right)["name"], 64))
	})
	if len(people) > MaxPeople {
		people = people[:MaxPeople]
	}
	out["people"] = people
	return Normalize(out, now)
}

// PersonName returns the bounded display name used in dependent snapshots.
func PersonName(person map[string]any) string { return strings.TrimSpace(Text(person["name"], 64)) }

// Find returns one person row by normalized ID.
func Find(rows []any, id string) (int, map[string]any) {
	id = ID(id)
	for index, raw := range rows {
		person := jsonutil.Map(raw)
		if ID(person["id"]) == id {
			return index, person
		}
	}
	return -1, nil
}

// NextRoster applies the current Dashboard Control People action without
// touching dependent app data. Core owns reconciliation around a delete.
func (s *Service) NextRoster(roster map[string]any, body map[string]any) (map[string]any, string, string, error) {
	op := strings.ToLower(Text(body["op"], 16))
	people := jsonutil.List(roster["people"])
	id := ID(body["id"])
	index, person := Find(people, id)
	now := s.now().Format(time.RFC3339)
	next := Default()
	next["revision"] = max(0, jsonutil.Int(roster["revision"], 0)) + 1
	switch op {
	case "add":
		name := Text(body["name"], 64)
		if name == "" {
			return nil, "", "", fmt.Errorf("person name is required")
		}
		if len(people) >= MaxPeople {
			return nil, "", "", fmt.Errorf("People supports up to %d people", MaxPeople)
		}
		for _, raw := range people {
			if strings.EqualFold(PersonName(jsonutil.Map(raw)), name) {
				return nil, "", "", fmt.Errorf("that person is already in the household roster")
			}
		}
		people = append(people, map[string]any{"id": fmt.Sprintf("person_%d", s.now().UnixNano()), "name": name, "state": "active", "createdAt": now, "updatedAt": now, "archivedAt": ""})
	case "rename":
		if person == nil {
			return nil, "", "", fmt.Errorf("person was not found")
		}
		name := Text(body["name"], 64)
		if name == "" {
			return nil, "", "", fmt.Errorf("person name is required")
		}
		for _, raw := range people {
			candidate := jsonutil.Map(raw)
			if ID(candidate["id"]) != id && strings.EqualFold(PersonName(candidate), name) {
				return nil, "", "", fmt.Errorf("that person is already in the household roster")
			}
		}
		person["name"], person["updatedAt"] = name, now
		people[index] = person
	case "archive", "restore":
		if person == nil {
			return nil, "", "", fmt.Errorf("person was not found")
		}
		person["state"], person["updatedAt"] = map[string]string{"archive": "archived", "restore": "active"}[op], now
		if op == "archive" {
			person["archivedAt"] = now
		} else {
			person["archivedAt"] = ""
		}
		people[index] = person
	case "delete":
		if person == nil {
			return nil, "", "", fmt.Errorf("person was not found")
		}
		people = append(people[:index], people[index+1:]...)
	default:
		return nil, "", "", fmt.Errorf("unknown People action")
	}
	next["people"] = people
	return Normalize(next, s.now()), op, id, nil
}

// DeleteTarget validates the current remove/reassign request against the
// post-removal roster, preserving existing Control response wording.
func DeleteTarget(next map[string]any, body map[string]any, removedID string) (string, error) {
	resolution := strings.ToLower(Text(body["resolution"], 16))
	if resolution == "" || resolution == "unassign" {
		return "", nil
	}
	if resolution != "reassign" {
		return "", fmt.Errorf("choose reassign or remove future assignments")
	}
	target := ID(body["reassignTo"])
	if target == "" || target == removedID {
		return "", fmt.Errorf("choose another active household person")
	}
	for _, raw := range jsonutil.List(next["people"]) {
		person := jsonutil.Map(raw)
		if ID(person["id"]) == target && person["state"] == "active" {
			return target, nil
		}
	}
	return "", fmt.Errorf("reassignment person was not found")
}
