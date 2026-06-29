// Package calendar owns Dash-Go calendar management, owned-feed presentation,
// local source visibility, generated default feeds, and bounded Calendar Trash.
// Event parsing, recurrence, source inspection, and cache construction remain in
// the dependency-free calendar/events child package.
package calendar

import "time"

const (
	TrashSchema        = 1
	TrashRetentionDays = 30
	TrashLimit         = 100
)

// Event is the stable generated-calendar event model used by household services
// and default-calendar generators before a deterministic ICS feed is written.
type Event struct {
	Date        time.Time
	Start       *time.Time
	End         *time.Time
	Summary     string
	Description string
	UID         string
	AppOwner    string
}

// Source is the canonical presentation metadata for a Dash-Go owned feed.
type Source struct {
	URL   string
	Name  string
	Color string
	Tag   string
	Owner string
}

// TrashRecord is the durable Calendar Trash record shape. Its JSON field names
// are part of the user-data compatibility contract.
type TrashRecord struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	TrashName  string `json:"trashName"`
	DeletedAt  string `json:"deletedAt"`
	PurgeAfter string `json:"purgeAfter"`
	WasEnabled bool   `json:"wasEnabled"`
	IsSymlink  bool   `json:"isSymlink"`
}

var palette = map[string]string{
	"green": "#8fc4a6", "blue": "#8bb4d4", "red": "#d99a9a",
	"gold": "#d9c074", "violet": "#9a8fb0", "purple": "#9a8fb0",
	"amber": "#cda76a", "teal": "#7fc4c4", "orange": "#d9a878",
	"slate": "#9aa0a6",
}

const defaultColor = "#7fd6a8"
