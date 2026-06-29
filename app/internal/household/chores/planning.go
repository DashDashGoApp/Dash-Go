package chores

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/jsonutil"
)

// LocalDateKey preserves the app's local-calendar semantics. The browser
// planner uses date-only keys as well; no UTC timestamp boundary is used for
// cadence or fairness decisions.
func LocalDateKey(value time.Time) string { return value.In(time.Local).Format("2006-01-02") }

func DateFromKey(key string) (time.Time, bool) {
	parsed, err := time.ParseInLocation("2006-01-02", DateKey(key), time.Local)
	return parsed, err == nil
}

func Cadence(chore map[string]any, now time.Time) map[string]any {
	row := jsonutil.Map(chore["cadence"])
	typ := Text(row["type"], 16)
	if typ != "daily" && typ != "weekdays" && typ != "weekly" && typ != "days" {
		typ = "daily"
	}
	return map[string]any{
		"type": typ, "day": clamp(jsonutil.Int(row["day"], 0), 0, 6),
		"every":      clamp(jsonutil.Int(row["every"], 1), 1, 365),
		"anchorDate": Anchor(row["anchorDate"], chore["createdAt"], now),
	}
}

func Due(chore map[string]any, key string, now time.Time) bool {
	date, ok := DateFromKey(key)
	if !ok {
		return false
	}
	cadence := Cadence(chore, now)
	switch cadence["type"] {
	case "daily":
		return true
	case "weekdays":
		return date.Weekday() != time.Saturday && date.Weekday() != time.Sunday
	case "weekly":
		return int(date.Weekday()) == jsonutil.Int(cadence["day"], 0)
	case "days":
		anchor, valid := DateFromKey(stringValue(cadence["anchorDate"]))
		if !valid || date.Before(anchor) {
			return false
		}
		delta := int(date.Sub(anchor).Hours() / 24)
		every := clamp(jsonutil.Int(cadence["every"], 1), 1, 365)
		return delta%every == 0
	default:
		return true
	}
}

func DueChores(payload map[string]any, key string, now time.Time) []map[string]any {
	out := []map[string]any{}
	for _, raw := range jsonutil.List(payload["chores"]) {
		chore := jsonutil.Map(raw)
		if Due(chore, key, now) {
			out = append(out, chore)
		}
	}
	return out
}

type fairnessIndex struct {
	key                  string
	monthPrefix          string
	peopleByID           map[string]map[string]any
	choreByID            map[string]map[string]any
	assignmentByChoreDay map[string]map[string]any
	eligibleByChore      map[string]map[string]bool
	monthLoad            map[string]int
	lastByPerson         map[string]string
	lastByChore          map[string]map[string]any
}

func buildFairnessIndex(payload map[string]any, key string) *fairnessIndex {
	index := &fairnessIndex{
		key: key, monthPrefix: strings.TrimSpace(key)[:min(len(strings.TrimSpace(key)), 7)],
		peopleByID: map[string]map[string]any{}, choreByID: map[string]map[string]any{},
		assignmentByChoreDay: map[string]map[string]any{}, eligibleByChore: map[string]map[string]bool{},
		monthLoad: map[string]int{}, lastByPerson: map[string]string{}, lastByChore: map[string]map[string]any{},
	}
	for _, raw := range jsonutil.List(payload["people"]) {
		person := jsonutil.Map(raw)
		if id := ID(person["id"]); id != "" {
			index.peopleByID[id] = person
		}
	}
	for _, raw := range jsonutil.List(payload["chores"]) {
		chore := jsonutil.Map(raw)
		id := ID(chore["id"])
		if id == "" {
			continue
		}
		index.choreByID[id] = chore
		allowed := map[string]bool{}
		for _, rawID := range jsonutil.List(chore["eligible"]) {
			if candidate := ID(rawID); candidate != "" {
				allowed[candidate] = true
			}
		}
		if len(allowed) == 0 {
			for candidate := range index.peopleByID {
				allowed[candidate] = true
			}
		}
		index.eligibleByChore[id] = allowed
	}
	for _, raw := range jsonutil.List(payload["assignments"]) {
		item := jsonutil.Map(raw)
		choreID, date := ID(item["choreId"]), DateKey(item["date"])
		if choreID == "" || date == "" {
			continue
		}
		index.assignmentByChoreDay[choreID+"|"+date] = item
		if Text(item["status"], 16) == "skipped" {
			continue
		}
		personID := ID(item["personId"])
		if strings.HasPrefix(date, index.monthPrefix) {
			effort := 1
			if chore := index.choreByID[choreID]; chore != nil {
				effort = clamp(jsonutil.Int(chore["effort"], 1), 1, 3)
			}
			index.monthLoad[personID] += effort
		}
		if date <= key && date > index.lastByPerson[personID] {
			index.lastByPerson[personID] = date
		}
		prior := index.lastByChore[choreID]
		if prior == nil || date > DateKey(prior["date"]) || (date == DateKey(prior["date"]) && ID(item["id"]) > ID(prior["id"])) {
			index.lastByChore[choreID] = item
		}
	}
	return index
}

func eligiblePeople(index *fairnessIndex, chore map[string]any) []map[string]any {
	allowed := index.eligibleByChore[ID(chore["id"])]
	out := []map[string]any{}
	for _, raw := range index.peopleByID {
		if len(allowed) == 0 || allowed[ID(raw["id"])] {
			out = append(out, raw)
		}
	}
	slices.SortFunc(out, func(left, right map[string]any) int { return strings.Compare(ID(left["id"]), ID(right["id"])) })
	return out
}

func StableHash(value string) uint32 {
	var hash uint32 = 2166136261
	for _, char := range value {
		hash ^= uint32(char)
		hash *= 16777619
	}
	return hash
}

func AssignmentID(chore, person map[string]any, key string) string {
	return "a-" + key + "-" + strconvBase36(StableHash(key+"|"+ID(chore["id"])+"|"+ID(person["id"])))
}

func strconvBase36(value uint32) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	if value == 0 {
		return "0"
	}
	out := make([]byte, 0, 8)
	for value > 0 {
		out = append(out, digits[value%36])
		value /= 36
	}
	for left, right := 0, len(out)-1; left < right; left, right = left+1, right-1 {
		out[left], out[right] = out[right], out[left]
	}
	return string(out)
}

func FairCandidate(payload map[string]any, chore map[string]any, key, excludeID string) map[string]any {
	return fairCandidate(buildFairnessIndex(payload, key), chore, key, excludeID)
}

func fairCandidate(index *fairnessIndex, chore map[string]any, key, excludeID string) map[string]any {
	candidates := eligiblePeople(index, chore)
	if len(candidates) == 0 {
		return nil
	}
	prior := index.lastByChore[ID(chore["id"])]
	pool := filterCandidates(candidates, func(person map[string]any) bool {
		return (prior == nil || ID(prior["personId"]) != ID(person["id"])) && ID(person["id"]) != excludeID
	})
	if len(pool) == 0 {
		pool = filterCandidates(candidates, func(person map[string]any) bool { return ID(person["id"]) != excludeID })
	}
	if len(pool) == 0 {
		pool = candidates
	}
	type candidate struct {
		person map[string]any
		load   int
		recent string
		seed   uint32
	}
	decorated := make([]candidate, 0, len(pool))
	for _, person := range pool {
		id := ID(person["id"])
		decorated = append(decorated, candidate{person: person, load: index.monthLoad[id], recent: index.lastByPerson[id], seed: StableHash(key + "|" + ID(chore["id"]) + "|" + id)})
	}
	slices.SortFunc(decorated, func(left, right candidate) int {
		if diff := cmp.Compare(left.load, right.load); diff != 0 {
			return diff
		}
		if diff := strings.Compare(left.recent, right.recent); diff != 0 {
			return diff
		}
		if diff := cmp.Compare(left.seed, right.seed); diff != 0 {
			return diff
		}
		return strings.Compare(ID(left.person["id"]), ID(right.person["id"]))
	})
	return decorated[0].person
}

func filterCandidates(values []map[string]any, keep func(map[string]any) bool) []map[string]any {
	out := []map[string]any{}
	for _, value := range values {
		if keep(value) {
			out = append(out, value)
		}
	}
	return out
}

func MakeAssignment(chore, person map[string]any, key, source string) map[string]any {
	if source == "" {
		source = "manual"
	}
	return map[string]any{
		"id": AssignmentID(chore, person, key), "date": key,
		"choreId": ID(chore["id"]), "choreName": Text(chore["name"], 96),
		"personId": ID(person["id"]), "personName": Text(person["name"], 64),
		"status": "assigned", "source": Text(source, 24),
	}
}

// GenerateDueAssignments is a deterministic server-side planning primitive.
// The existing browser keeps its visual spin and batch planner; this function
// gives the bounded service the same cadence/fairness semantics for callers
// that need a durable schedule projection without depending on browser state.
func (s *Service) GenerateDueAssignments(payload map[string]any, key, source string) []any {
	payload = NormalizeAt(payload, s.Now())
	index := buildFairnessIndex(payload, key)
	planned := []any{}
	for _, chore := range DueChores(payload, key, s.Now()) {
		if index.assignmentByChoreDay[ID(chore["id"])+"|"+key] != nil {
			continue
		}
		winner := fairCandidate(index, chore, key, "")
		if winner == nil {
			continue
		}
		assignment := MakeAssignment(chore, winner, key, source)
		planned = append(planned, assignment)
		index.assignmentByChoreDay[ID(chore["id"])+"|"+key] = assignment
		if strings.HasPrefix(key, index.monthPrefix) {
			index.monthLoad[ID(winner["id"])] += clamp(jsonutil.Int(chore["effort"], 1), 1, 3)
		}
		if key <= index.key && key > index.lastByPerson[ID(winner["id"])] {
			index.lastByPerson[ID(winner["id"])] = key
		}
		index.lastByChore[ID(chore["id"])] = assignment
	}
	return planned
}
