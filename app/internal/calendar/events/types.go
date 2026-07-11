// Package events owns ICS parsing, recurrence expansion, calendar source
// inspection, and the durable event-cache pipeline. It intentionally receives
// calendar visibility and owned-feed policy through narrow callbacks: those
// policies remain in core until the calendar-management extraction.
package events

import "time"

// CacheVersion advances when recurrence expansion semantics change. A previous
// cache remains structurally readable, but must be rebuilt before it can be
// reused with the current parser.
const CacheVersion = 11
const FingerprintVersion = 9
const maxRecurrenceSteps = 50000

// CalendarSource is the stable source descriptor persisted inside
// events.cache.json. Field names and JSON tags intentionally retain the
// established cache format.
type CalendarSource struct {
	URL   string `json:"url"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Tag   string `json:"tag"`
	Owner string `json:"owner,omitempty"`
}

// SourceMeta is the cache fingerprint/status descriptor. Its field ordering is
// retained because the compact JSON cache and fingerprint are compatibility
// surfaces.
type SourceMeta struct {
	URL       string   `json:"url"`
	Name      string   `json:"name"`
	Color     string   `json:"color"`
	Tag       string   `json:"tag"`
	Owner     string   `json:"owner,omitempty"`
	Path      string   `json:"path"`
	Exists    bool     `json:"exists"`
	MtimeMs   *int64   `json:"mtimeMs"`
	MtimeNs   *int64   `json:"mtimeNs,omitempty"`
	Size      *int64   `json:"size"`
	SHA256    *string  `json:"sha256"`
	HashedAt  *int64   `json:"hashedAt,omitempty"`
	RealPath  string   `json:"realPath"`
	IsSymlink bool     `json:"isSymlink"`
	AgeHours  *float64 `json:"ageHours,omitempty"`
	HashError string   `json:"hashError,omitempty"`
}

type rdate struct {
	Start    time.Time
	DateOnly bool
}

// ICSEvent is the parsed internal event representation used before cache
// serialization. It is exported only for the focused core compatibility facade
// and tests; parser/recurrence behavior remains owned by this package.
type ICSEvent struct {
	Cal             CalendarSource
	Start           time.Time
	End             *time.Time
	AllDay          bool
	Title           string
	Desc            string
	Location        string
	UID             string
	RRule           string
	Rdates          []rdate
	Exdates         map[int64]bool
	ExdateDays      map[string]bool
	RecurID         *int64
	RecurIDDay      string
	RecurIDDateOnly bool
	Skip            map[int64]bool
	SkipDays        map[string]bool
	Recur           bool
	Seq             int
	Status          string
	LastModified    time.Time
	DTStamp         time.Time
	Cancelled       bool
	AppOwner        string
	Meta            map[string]string
	HasScheduling   bool
	zone            *calendarZone
}

// SourceDiagnostics records bounded per-calendar parser evidence without
// retaining event bodies. It makes malformed or superseded feeds diagnosable
// while keeping the normal dashboard payload small.
type SourceDiagnostics struct {
	URL                  string `json:"url"`
	Name                 string `json:"name"`
	ComponentsFound      int    `json:"componentsFound"`
	EventsAccepted       int    `json:"eventsAccepted"`
	InvalidEventsDropped int    `json:"invalidEventsDropped"`
	CancelledEvents      int    `json:"cancelledEvents"`
	SupersededRevisions  int    `json:"supersededRevisions"`
	ExpandedOccurrences  int    `json:"expandedOccurrences"`
	ParseDurationMs      int64  `json:"parseDurationMs"`
	Error                string `json:"error,omitempty"`
}

// CacheOutput is serialized byte-for-byte through the established compact JSON
// writer. Keep field order and JSON tags stable.
type CacheOutput struct {
	Version            int                 `json:"version"`
	FingerprintVersion int                 `json:"fingerprintVersion"`
	GeneratedAt        int64               `json:"generatedAt"`
	WindowStart        int64               `json:"windowStart"`
	WindowEnd          int64               `json:"windowEnd"`
	Sources            []SourceMeta        `json:"sources"`
	Issues             []string            `json:"issues"`
	Diagnostics        []SourceDiagnostics `json:"diagnostics"`
	Events             []map[string]any    `json:"events"`
}
