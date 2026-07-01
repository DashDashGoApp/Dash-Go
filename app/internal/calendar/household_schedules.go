package calendar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/DashDashGoApp/Dash-Go/app/internal/fileio"
)

// HouseholdSchedulesSchema is the durable, local-only configuration contract
// for generated Paydays, Trash Pickup, and Recycling Pickup feeds.
const HouseholdSchedulesSchema = 1

var (
	reScheduleID  = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,47}$`)
	reScheduleISO = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

type HouseholdSchedules struct {
	Schema    int                `json:"schema"`
	Paydays   []PaydayRule       `json:"paydays"`
	Pickups   []PickupRule       `json:"pickups"`
	Overrides []ScheduleOverride `json:"overrides"`
}

type ScheduleAdjustment struct {
	Mode          string   `json:"mode"`
	Days          int      `json:"days,omitempty"`
	Weekends      bool     `json:"weekends,omitempty"`
	HolidayLayers []string `json:"holidayLayers,omitempty"`
	WeekHoliday   bool     `json:"weekHoliday,omitempty"`
}

type PaydayRule struct {
	ID         string             `json:"id"`
	Label      string             `json:"label"`
	Enabled    bool               `json:"enabled"`
	Kind       string             `json:"kind"`
	Start      string             `json:"start,omitempty"`
	EveryWeeks int                `json:"everyWeeks,omitempty"`
	Days       []int              `json:"days,omitempty"`
	Nth        int                `json:"nth,omitempty"`
	Weekday    string             `json:"weekday,omitempty"`
	Adjustment ScheduleAdjustment `json:"adjustment"`
}

type PickupRule struct {
	ID         string             `json:"id"`
	Label      string             `json:"label"`
	Enabled    bool               `json:"enabled"`
	Weekday    string             `json:"weekday"`
	EveryWeeks int                `json:"everyWeeks"`
	Start      string             `json:"start,omitempty"`
	Adjustment ScheduleAdjustment `json:"adjustment"`
}

type ScheduleOverride struct {
	RuleID      string `json:"ruleId"`
	NominalDate string `json:"nominalDate"`
	Action      string `json:"action"`
	ActualDate  string `json:"actualDate,omitempty"`
}

// ScheduleOverrideResult gives the HTTP layer the resolved safety outcome
// without making the browser reverse-engineer generated calendar feeds.
type ScheduleOverrideResult struct {
	Schedules   HouseholdSchedules
	RuleID      string
	NominalDate string
	ActualDate  string
	Collision   bool
}

type HolidayLayer struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func defaultHouseholdSchedules() HouseholdSchedules {
	return HouseholdSchedules{Schema: HouseholdSchedulesSchema, Paydays: []PaydayRule{}, Pickups: []PickupRule{}, Overrides: []ScheduleOverride{}}
}

func (s *Service) householdSchedulesPath() string {
	if s == nil {
		return ""
	}
	if strings.TrimSpace(s.householdSchedulesFile) != "" {
		return s.householdSchedulesFile
	}
	return filepath.Join(s.dashDir, "config", "household-schedules.json")
}

func scheduleDate(value string) (time.Time, bool) {
	if !reScheduleISO.MatchString(strings.TrimSpace(value)) {
		return time.Time{}, false
	}
	day, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	return day, err == nil
}

func scheduleDateKey(day time.Time) string { return day.UTC().Format("2006-01-02") }

func normalizeLayerID(value string) string {
	return strings.NewReplacer("_", "-", " ", "-", ",", "-").Replace(strings.ToLower(strings.TrimSpace(value)))
}

func normalizeScheduleAdjustment(in ScheduleAdjustment, pickup bool) (ScheduleAdjustment, error) {
	mode := strings.ToLower(strings.TrimSpace(in.Mode))
	if mode == "" {
		mode = "none"
	}
	allowed := map[string]bool{"none": true, "previous-business-day": true, "next-business-day": true, "shift-forward": true, "shift-backward": true}
	if !allowed[mode] {
		return ScheduleAdjustment{}, fmt.Errorf("unknown schedule adjustment")
	}
	if (mode == "shift-forward" || mode == "shift-backward") && !pickup {
		return ScheduleAdjustment{}, fmt.Errorf("paydays use a business-day adjustment")
	}
	out := ScheduleAdjustment{Mode: mode, Weekends: in.Weekends, WeekHoliday: in.WeekHoliday}
	if mode == "shift-forward" || mode == "shift-backward" {
		if in.Days < 1 || in.Days > 7 {
			return ScheduleAdjustment{}, fmt.Errorf("schedule shift days must be between 1 and 7")
		}
		out.Days = in.Days
	}
	seen := map[string]bool{}
	for _, raw := range in.HolidayLayers {
		id := normalizeLayerID(raw)
		if id == "" || seen[id] {
			continue
		}
		if id != "civil" && id != "jewish" && id != "islamic" && id != "christian" && id != "orthodox" && id != "hindu" {
			return ScheduleAdjustment{}, fmt.Errorf("unknown holiday layer")
		}
		seen[id] = true
		out.HolidayLayers = append(out.HolidayLayers, id)
	}
	sort.Strings(out.HolidayLayers)
	return out, nil
}

func normalizePaydayRule(in PaydayRule) (PaydayRule, error) {
	out := in
	out.ID = strings.ToLower(strings.TrimSpace(out.ID))
	out.Label = strings.TrimSpace(out.Label)
	out.Kind = strings.ToLower(strings.TrimSpace(out.Kind))
	if !reScheduleID.MatchString(out.ID) {
		return PaydayRule{}, fmt.Errorf("payday rule id is invalid")
	}
	if out.Label == "" || len([]rune(out.Label)) > 64 {
		return PaydayRule{}, fmt.Errorf("payday label must be 1 to 64 characters")
	}
	if out.Kind != "every-weeks" && out.Kind != "monthly-dates" && out.Kind != "nth-weekday" {
		return PaydayRule{}, fmt.Errorf("unknown payday schedule")
	}
	adj, err := normalizeScheduleAdjustment(out.Adjustment, false)
	if err != nil {
		return PaydayRule{}, err
	}
	out.Adjustment = adj
	switch out.Kind {
	case "every-weeks":
		if _, ok := scheduleDate(out.Start); !ok {
			return PaydayRule{}, fmt.Errorf("every-weeks payday needs a known payday date")
		}
		if out.EveryWeeks < 1 || out.EveryWeeks > 52 {
			return PaydayRule{}, fmt.Errorf("payday interval must be between 1 and 52 weeks")
		}
		out.Days, out.Nth, out.Weekday = nil, 0, ""
	case "monthly-dates":
		seen := map[int]bool{}
		days := make([]int, 0, len(out.Days))
		for _, day := range out.Days {
			if day < 1 || day > 31 || seen[day] {
				continue
			}
			seen[day] = true
			days = append(days, day)
		}
		out.Days = days
		sort.Ints(out.Days)
		if len(out.Days) == 0 || len(out.Days) > 12 {
			return PaydayRule{}, fmt.Errorf("choose between 1 and 12 monthly payday dates")
		}
		out.Start, out.EveryWeeks, out.Nth, out.Weekday = "", 0, 0, ""
	case "nth-weekday":
		if out.Nth != 1 && out.Nth != 2 && out.Nth != 3 && out.Nth != 4 && out.Nth != -1 {
			return PaydayRule{}, fmt.Errorf("choose first through fourth or last weekday")
		}
		if weekdayIndex(out.Weekday) < 0 {
			return PaydayRule{}, fmt.Errorf("choose a weekday for this payday")
		}
		out.Start, out.EveryWeeks, out.Days = "", 0, nil
	}
	return out, nil
}

func normalizePickupRule(in PickupRule) (PickupRule, error) {
	out := in
	out.ID = strings.ToLower(strings.TrimSpace(out.ID))
	out.Label = strings.TrimSpace(out.Label)
	if out.ID != "trash" && out.ID != "recycling" {
		return PickupRule{}, fmt.Errorf("pickup rule id must be trash or recycling")
	}
	if out.Label == "" || len([]rune(out.Label)) > 64 {
		return PickupRule{}, fmt.Errorf("pickup label must be 1 to 64 characters")
	}
	if weekdayIndex(out.Weekday) < 0 {
		return PickupRule{}, fmt.Errorf("pickup weekday is invalid")
	}
	if out.EveryWeeks < 1 || out.EveryWeeks > 52 {
		return PickupRule{}, fmt.Errorf("pickup interval must be between 1 and 52 weeks")
	}
	if out.Start != "" {
		if _, ok := scheduleDate(out.Start); !ok {
			return PickupRule{}, fmt.Errorf("pickup start date must use YYYY-MM-DD")
		}
	}
	adj, err := normalizeScheduleAdjustment(out.Adjustment, true)
	if err != nil {
		return PickupRule{}, err
	}
	out.Adjustment = adj
	return out, nil
}

func normalizeOverrides(in []ScheduleOverride, known map[string]bool) ([]ScheduleOverride, error) {
	out := make([]ScheduleOverride, 0, len(in))
	seen := map[string]bool{}
	for _, row := range in {
		row.RuleID = strings.ToLower(strings.TrimSpace(row.RuleID))
		row.Action = strings.ToLower(strings.TrimSpace(row.Action))
		if !known[row.RuleID] || !reScheduleID.MatchString(row.RuleID) || !scheduleDateKeyValid(row.NominalDate) {
			return nil, fmt.Errorf("schedule override is invalid")
		}
		if row.Action != "move" && row.Action != "skip" {
			return nil, fmt.Errorf("schedule override action is invalid")
		}
		if row.Action == "move" && !scheduleDateKeyValid(row.ActualDate) {
			return nil, fmt.Errorf("moved schedule date is invalid")
		}
		if row.Action == "skip" {
			row.ActualDate = ""
		}
		key := row.RuleID + "|" + row.NominalDate
		if seen[key] {
			return nil, fmt.Errorf("duplicate schedule override")
		}
		seen[key] = true
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RuleID+out[i].NominalDate < out[j].RuleID+out[j].NominalDate })
	return out, nil
}

func scheduleDateKeyValid(value string) bool { _, ok := scheduleDate(value); return ok }

// normalizeOverridesForLoad keeps strict shape validation but discards only
// overrides whose rule no longer exists. A stale adjustment must not prevent
// the complete generated-calendar job from loading its otherwise valid model.
func normalizeOverridesForLoad(in []ScheduleOverride, known map[string]bool) ([]ScheduleOverride, []string, error) {
	retained := make([]ScheduleOverride, 0, len(in))
	dropped := []string{}
	for _, row := range in {
		ruleID := strings.ToLower(strings.TrimSpace(row.RuleID))
		if !known[ruleID] {
			dropped = append(dropped, strings.TrimSpace(row.RuleID)+"|"+strings.TrimSpace(row.NominalDate))
			continue
		}
		retained = append(retained, row)
	}
	normalized, err := normalizeOverrides(retained, known)
	return normalized, dropped, err
}

func normalizeHouseholdSchedules(in HouseholdSchedules) (HouseholdSchedules, error) {
	normalized, _, err := normalizeHouseholdSchedulesForLoadPolicy(in, false)
	return normalized, err
}

func normalizeHouseholdSchedulesForLoad(in HouseholdSchedules) (HouseholdSchedules, []string, error) {
	return normalizeHouseholdSchedulesForLoadPolicy(in, true)
}

func normalizeHouseholdSchedulesForLoadPolicy(in HouseholdSchedules, dropOrphans bool) (HouseholdSchedules, []string, error) {
	out := defaultHouseholdSchedules()
	if in.Schema != 0 && in.Schema != HouseholdSchedulesSchema {
		return out, nil, fmt.Errorf("household schedules schema is not supported")
	}
	known := map[string]bool{}
	for _, rule := range in.Paydays {
		next, err := normalizePaydayRule(rule)
		if err != nil {
			return out, nil, err
		}
		if known[next.ID] {
			return out, nil, fmt.Errorf("schedule ids must be unique")
		}
		known[next.ID] = true
		out.Paydays = append(out.Paydays, next)
	}
	for _, rule := range in.Pickups {
		next, err := normalizePickupRule(rule)
		if err != nil {
			return out, nil, err
		}
		if known[next.ID] {
			return out, nil, fmt.Errorf("schedule ids must be unique")
		}
		known[next.ID] = true
		out.Pickups = append(out.Pickups, next)
	}
	if len(out.Paydays) > 24 || len(out.Pickups) > 8 {
		return out, nil, fmt.Errorf("too many household schedule rules")
	}
	var err error
	dropped := []string{}
	if dropOrphans {
		out.Overrides, dropped, err = normalizeOverridesForLoad(in.Overrides, known)
	} else {
		out.Overrides, err = normalizeOverrides(in.Overrides, known)
	}
	if err != nil {
		return out, dropped, err
	}
	if len(out.Overrides) > 512 {
		return out, dropped, fmt.Errorf("too many household schedule adjustments")
	}
	return out, dropped, nil
}

func legacyHouseholdSchedules(values map[string]string) HouseholdSchedules {
	out := defaultHouseholdSchedules()
	legacyAdjustment := ScheduleAdjustment{Mode: "none"}
	if values["PICKUP_HOLIDAY_SHIFT"] == "1" {
		mode := "shift-forward"
		if strings.HasPrefix(strings.ToLower(values["PICKUP_SHIFT"]), "back") {
			mode = "shift-backward"
		}
		legacyAdjustment = ScheduleAdjustment{Mode: mode, Days: atoiClamp(values["PICKUP_SHIFT_DAYS"], 1, 1, 7), HolidayLayers: []string{"civil"}, WeekHoliday: true}
	}
	if weekdayIndex(values["TRASH_WEEKDAY"]) >= 0 {
		out.Pickups = append(out.Pickups, PickupRule{ID: "trash", Label: "Trash pickup", Enabled: true, Weekday: values["TRASH_WEEKDAY"], EveryWeeks: 1, Adjustment: legacyAdjustment})
	}
	if weekdayIndex(values["RECYCLING_WEEKDAY"]) >= 0 {
		out.Pickups = append(out.Pickups, PickupRule{ID: "recycling", Label: "Recycling pickup", Enabled: true, Weekday: values["RECYCLING_WEEKDAY"], EveryWeeks: atoiClamp(values["RECYCLING_EVERY_WEEKS"], 2, 1, 52), Adjustment: legacyAdjustment})
	}
	switch strings.ToLower(strings.TrimSpace(values["PAYDAY_MODE"])) {
	case "weekly", "biweekly":
		if scheduleDateKeyValid(values["PAYDAY_START"]) {
			every := 1
			if strings.EqualFold(strings.TrimSpace(values["PAYDAY_MODE"]), "biweekly") {
				every = 2
			}
			out.Paydays = append(out.Paydays, PaydayRule{ID: "payday", Label: "Payday", Enabled: true, Kind: "every-weeks", Start: values["PAYDAY_START"], EveryWeeks: every, Adjustment: ScheduleAdjustment{Mode: "none"}})
		}
	case "monthly":
		out.Paydays = append(out.Paydays, PaydayRule{ID: "payday", Label: "Payday", Enabled: true, Kind: "monthly-dates", Days: []int{atoiClamp(values["PAYDAY_DAY"], 1, 1, 28)}, Adjustment: ScheduleAdjustment{Mode: "none"}})
	}
	return out
}

func (s *Service) loadHouseholdSchedulesLocked(values map[string]string) (HouseholdSchedules, bool, error) {
	path := s.householdSchedulesPath()
	if path == "" {
		return defaultHouseholdSchedules(), false, nil
	}
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return legacyHouseholdSchedules(values), true, nil
	}
	if err != nil {
		return HouseholdSchedules{}, false, err
	}
	var raw HouseholdSchedules
	if err := json.Unmarshal(body, &raw); err != nil {
		return HouseholdSchedules{}, false, fmt.Errorf("household schedules are malformed")
	}
	normalized, dropped, err := normalizeHouseholdSchedulesForLoad(raw)
	if err != nil {
		return HouseholdSchedules{}, false, err
	}
	if len(dropped) > 0 {
		s.appendLog("household-schedules.log", fmt.Sprintf("%s: dropped orphaned one-time schedule adjustments: %s\n", s.now().Format(time.ANSIC), strings.Join(dropped, ", ")))
	}
	return normalized, false, nil
}

func (s *Service) householdSchedulesLocked() (HouseholdSchedules, bool, error) {
	return s.loadHouseholdSchedulesLocked(s.DefaultConfig())
}

func (s *Service) HouseholdSchedules() (HouseholdSchedules, bool, error) {
	if s == nil {
		return defaultHouseholdSchedules(), false, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.householdSchedulesLocked()
}

func scheduleSnapshot(path string) ([]byte, bool, error) {
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return body, err == nil, err
}

func restoreScheduleSnapshot(path string, body []byte, existed bool) error {
	if existed {
		return fileio.WriteAtomic(path, body, 0644)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Service) writeHouseholdSchedulesLocked(next HouseholdSchedules) error {
	if err := os.MkdirAll(filepath.Dir(s.householdSchedulesPath()), 0755); err != nil {
		return err
	}
	return fileio.WriteJSON(s.householdSchedulesPath(), next)
}

func (s *Service) writeHouseholdScheduleFeedsLocked(next HouseholdSchedules, start, end time.Time) error {
	holidayDates := s.HolidayDatesByLayer(nextHolidayLayers(next))
	feeds := householdScheduleFeeds(next, start, end, holidayDates)
	paths := map[string]string{
		"trash":     filepath.Join(s.calendarDir, "trash.amber.ics"),
		"recycling": filepath.Join(s.calendarDir, "recycling.teal.ics"),
		"payday":    filepath.Join(s.calendarDir, "payday.violet.pay.ics"),
	}
	for key, path := range paths {
		events := feeds[key]
		if len(events) == 0 {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		name := map[string]string{"trash": "Trash Pickup", "recycling": "Recycling Pickup", "payday": "Paydays"}[key]
		if err := WriteICSFile(path, name, events); err != nil {
			return err
		}
	}
	return nil
}

func nextHolidayLayers(next HouseholdSchedules) []string {
	seen := map[string]bool{}
	for _, rule := range next.Paydays {
		for _, layer := range rule.Adjustment.HolidayLayers {
			seen[layer] = true
		}
	}
	for _, rule := range next.Pickups {
		for _, layer := range rule.Adjustment.HolidayLayers {
			seen[layer] = true
		}
	}
	out := make([]string, 0, len(seen))
	for layer := range seen {
		out = append(out, layer)
	}
	sort.Strings(out)
	return out
}

func (s *Service) saveHouseholdSchedulesLocked(next HouseholdSchedules) error {
	oldConfig, oldConfigExists, err := scheduleSnapshot(s.householdSchedulesPath())
	if err != nil {
		return err
	}
	feedPaths := []string{filepath.Join(s.calendarDir, "trash.amber.ics"), filepath.Join(s.calendarDir, "recycling.teal.ics"), filepath.Join(s.calendarDir, "payday.violet.pay.ics")}
	type snap struct {
		path    string
		body    []byte
		existed bool
	}
	snaps := make([]snap, 0, len(feedPaths))
	for _, path := range feedPaths {
		body, existed, err := scheduleSnapshot(path)
		if err != nil {
			return err
		}
		snaps = append(snaps, snap{path, body, existed})
	}
	rollback := func() {
		_ = restoreScheduleSnapshot(s.householdSchedulesPath(), oldConfig, oldConfigExists)
		for _, saved := range snaps {
			_ = restoreScheduleSnapshot(saved.path, saved.body, saved.existed)
		}
		_ = s.generateManifestLocked(s.outputSnapshot())
	}
	if err := s.writeHouseholdSchedulesLocked(next); err != nil {
		return err
	}
	today := s.now()
	start := DateOnly(today.Year(), 1, 1)
	end := DateOnly(today.Year()+3, 1, 1)
	if err := s.writeHouseholdScheduleFeedsLocked(next, start, end); err != nil {
		rollback()
		return err
	}
	if err := s.generateManifestLocked(s.outputSnapshot()); err != nil {
		rollback()
		return err
	}
	return nil
}

func (s *Service) SaveHouseholdSchedules(input HouseholdSchedules) (HouseholdSchedules, error) {
	next, err := normalizeHouseholdSchedules(input)
	if err != nil {
		return HouseholdSchedules{}, err
	}
	if s == nil {
		return next, nil
	}
	s.mu.Lock()
	err = s.saveHouseholdSchedulesLocked(next)
	s.mu.Unlock()
	if err != nil {
		return HouseholdSchedules{}, err
	}
	return next, nil
}

func scheduleGenerationWindow(now time.Time) (time.Time, time.Time) {
	start := DateOnly(now.Year(), time.January, 1)
	return start, DateOnly(now.Year()+3, time.January, 1)
}

func validateScheduleMove(change ScheduleOverride, start, end time.Time) (time.Time, error) {
	nominal, nominalOK := scheduleDate(change.NominalDate)
	actual, actualOK := scheduleDate(change.ActualDate)
	if !nominalOK || !actualOK {
		return time.Time{}, fmt.Errorf("moved schedule date is invalid")
	}
	if nominal.Before(start) || !nominal.Before(end) {
		return time.Time{}, fmt.Errorf("scheduled occurrence is outside the generated calendar window")
	}
	if actual.Before(start) || !actual.Before(end) {
		return time.Time{}, fmt.Errorf("moved date must remain inside the generated calendar window (%s through %s)", scheduleDateKey(start), scheduleDateKey(end.AddDate(0, 0, -1)))
	}
	days := int(actual.Sub(nominal) / (24 * time.Hour))
	if days < -90 || days > 90 {
		return time.Time{}, fmt.Errorf("move a scheduled occurrence no more than 90 days earlier or later")
	}
	return actual, nil
}

func scheduleOverrideCollides(next HouseholdSchedules, ruleID, nominalDate string, actual, start, end time.Time, holidays map[string]map[string]bool) bool {
	for _, events := range householdScheduleFeeds(next, start, end, holidays) {
		for _, event := range events {
			if event.Meta["X-DASHGO-SCHEDULE-RULE-ID"] != ruleID || event.Meta["X-DASHGO-NOMINAL-DATE"] == nominalDate {
				continue
			}
			if scheduleDateKey(event.Date) == scheduleDateKey(actual) {
				return true
			}
		}
	}
	return false
}

func (s *Service) SetHouseholdScheduleOverride(change ScheduleOverride) (HouseholdSchedules, error) {
	result, err := s.SetHouseholdScheduleOverrideWithResult(change)
	if err != nil {
		return HouseholdSchedules{}, err
	}
	return result.Schedules, nil
}

func (s *Service) SetHouseholdScheduleOverrideWithResult(change ScheduleOverride) (ScheduleOverrideResult, error) {
	if s == nil {
		return ScheduleOverrideResult{}, fmt.Errorf("household schedules unavailable")
	}
	change.RuleID = strings.ToLower(strings.TrimSpace(change.RuleID))
	change.Action = strings.ToLower(strings.TrimSpace(change.Action))
	change.NominalDate = strings.TrimSpace(change.NominalDate)
	change.ActualDate = strings.TrimSpace(change.ActualDate)
	result := ScheduleOverrideResult{RuleID: change.RuleID, NominalDate: change.NominalDate}
	s.mu.Lock()
	current, _, err := s.householdSchedulesLocked()
	if err != nil {
		s.mu.Unlock()
		return ScheduleOverrideResult{}, err
	}
	if !scheduleDateKeyValid(change.NominalDate) {
		s.mu.Unlock()
		return ScheduleOverrideResult{}, fmt.Errorf("scheduled date is invalid")
	}
	start, end := scheduleGenerationWindow(s.now())
	actual := time.Time{}
	if change.Action == "move" {
		actual, err = validateScheduleMove(change, start, end)
		if err != nil {
			s.mu.Unlock()
			return ScheduleOverrideResult{}, err
		}
		result.ActualDate = scheduleDateKey(actual)
	} else if change.Action == "clear" {
		result.ActualDate = change.NominalDate
	}
	if change.Action == "clear" {
		current.Overrides = slices.DeleteFunc(current.Overrides, func(row ScheduleOverride) bool {
			return row.RuleID == change.RuleID && row.NominalDate == change.NominalDate
		})
	} else {
		found := false
		for i := range current.Overrides {
			if current.Overrides[i].RuleID == change.RuleID && current.Overrides[i].NominalDate == change.NominalDate {
				current.Overrides[i] = change
				found = true
				break
			}
		}
		if !found {
			current.Overrides = append(current.Overrides, change)
		}
	}
	next, err := normalizeHouseholdSchedules(current)
	if err == nil && change.Action == "move" {
		holidays := s.HolidayDatesByLayer(nextHolidayLayers(next))
		result.Collision = scheduleOverrideCollides(next, change.RuleID, change.NominalDate, actual, start, end, holidays)
	}
	if err == nil {
		err = s.saveHouseholdSchedulesLocked(next)
	}
	s.mu.Unlock()
	if err != nil {
		return ScheduleOverrideResult{}, err
	}
	result.Schedules = next
	return result, nil
}

func (s *Service) AvailableHolidayLayers() []HolidayLayer {
	if s == nil {
		return []HolidayLayer{}
	}
	specs := []HolidayLayer{{"civil", "Public holidays"}, {"jewish", "Jewish observances"}, {"islamic", "Islamic observances"}, {"christian", "Christian observances"}, {"orthodox", "Orthodox observances"}, {"hindu", "Hindu observances"}}
	out := []HolidayLayer{}
	for _, spec := range specs {
		path := s.holidayLayerPath(spec.ID)
		if path != "" {
			if _, err := os.Stat(path); err == nil {
				out = append(out, spec)
			}
		}
	}
	return out
}

func (s *Service) holidayLayerPath(id string) string {
	file := map[string]string{
		"civil": "holidays.blue.holiday.ics", "jewish": "jewish-holidays.violet.holiday.ics", "islamic": "islamic-holidays.teal.holiday.ics", "christian": "christian-holidays.gold.holiday.ics", "orthodox": "orthodox-holidays.slate.holiday.ics", "hindu": "hindu-holidays.amber.holiday.ics",
	}[id]
	if file == "" {
		return ""
	}
	return filepath.Join(s.calendarDir, file)
}

func (s *Service) HolidayDatesByLayer(layers []string) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, raw := range layers {
		id := normalizeLayerID(raw)
		path := s.holidayLayerPath(id)
		if path == "" {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		dates := map[string]bool{}
		for _, line := range strings.Split(string(body), "\n") {
			if matches := reHolidayDate.FindStringSubmatch(strings.TrimSpace(line)); len(matches) == 2 {
				dates[matches[1]] = true
			}
		}
		out[id] = dates
	}
	return out
}

func (s *Service) HolidayDatesForLayers(layers []string) map[string]bool {
	out := map[string]bool{}
	for _, dates := range s.HolidayDatesByLayer(layers) {
		for day := range dates {
			out[day] = true
		}
	}
	return out
}
