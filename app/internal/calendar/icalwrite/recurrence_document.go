package icalwrite

import (
	"errors"
	"strings"
)

// eventComponent is a top-level VEVENT inside a vdir item. A recurring item
// may legitimately contain the recurrence master and one or more detached
// overrides. The ordinary one-time editor deliberately still requires one
// VEVENT; these helpers are the narrow recurrence-aware path.
type eventComponent struct {
	begin        int
	end          int
	direct       map[int]bool
	uid          string
	recurrenceID string
	recurring    bool
	scheduling   bool
}

func documentComponents(lines []string) ([]eventComponent, error) {
	depth, eventDepth, begin := 0, 0, -1
	var direct map[int]bool
	components := []eventComponent{}
	for i, line := range lines {
		name := propName(line)
		value := strings.ToUpper(strings.TrimSpace(propValue(line)))
		switch {
		case name == "BEGIN":
			depth++
			if value == "VEVENT" {
				if begin >= 0 {
					return nil, errors.New("nested VEVENT is invalid")
				}
				begin, eventDepth = i, depth
				direct = map[int]bool{}
			}
		case name == "END":
			if value == "VEVENT" && begin >= 0 && depth == eventDepth {
				component := eventComponent{begin: begin, end: i, direct: direct}
				for index := range direct {
					switch propName(lines[index]) {
					case "UID":
						component.uid = strings.TrimSpace(propValue(lines[index]))
					case "RECURRENCE-ID":
						component.recurrenceID = strings.TrimSpace(propValue(lines[index]))
					case "RRULE", "RDATE":
						component.recurring = true
					case "ORGANIZER", "ATTENDEE":
						component.scheduling = true
					}
				}
				components = append(components, component)
				begin, eventDepth, direct = -1, 0, nil
			}
			if depth > 0 {
				depth--
			}
		case begin >= 0 && depth == eventDepth:
			direct[i] = true
		}
	}
	if begin >= 0 || len(components) == 0 {
		return nil, errors.New("VEVENT document is malformed")
	}
	return components, nil
}

func parseDocument(src string) ([]string, []eventComponent, error) {
	lines := unfold(src)
	components, err := documentComponents(lines)
	if err != nil {
		return nil, nil, err
	}
	return lines, components, nil
}

func componentLines(lines []string, component eventComponent) []string {
	return append([]string(nil), lines[component.begin:component.end+1]...)
}

func componentUIDs(components []eventComponent) map[string]bool {
	out := map[string]bool{}
	for _, component := range components {
		if component.uid != "" {
			out[component.uid] = true
		}
	}
	return out
}

// HasUID reports whether this vdir object represents one logical UID. It is
// intentionally strict: an aggregate file with unrelated UIDs is never a
// safe write target.
func HasUID(src, uid string) bool {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return false
	}
	_, components, err := parseDocument(src)
	if err != nil {
		return false
	}
	for _, component := range components {
		if component.uid != uid {
			return false
		}
	}
	return len(components) > 0
}

func documentUID(src string) string {
	_, components, err := parseDocument(src)
	if err != nil {
		return ""
	}
	unique := componentUIDs(components)
	if len(unique) != 1 {
		return ""
	}
	for uid := range unique {
		return uid
	}
	return ""
}

func documentHasScheduling(src string) bool {
	_, components, err := parseDocument(src)
	if err != nil {
		return false
	}
	for _, component := range components {
		if component.scheduling {
			return true
		}
	}
	return false
}

func documentMaster(src string, uid string) ([]string, []eventComponent, eventComponent, error) {
	lines, components, err := parseDocument(src)
	if err != nil {
		return nil, nil, eventComponent{}, err
	}
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return nil, nil, eventComponent{}, errors.New("event UID required")
	}
	var master *eventComponent
	for i := range components {
		component := components[i]
		if component.uid != uid {
			return nil, nil, eventComponent{}, errors.New("calendar item has mixed event identifiers")
		}
		if component.recurrenceID == "" {
			if master != nil {
				return nil, nil, eventComponent{}, errors.New("calendar item has multiple recurrence masters")
			}
			copy := component
			master = &copy
		}
	}
	if master == nil {
		return nil, nil, eventComponent{}, errors.New("recurrence master is no longer available")
	}
	return lines, components, *master, nil
}

func replaceComponent(lines []string, component eventComponent, replacement []string) []string {
	out := make([]string, 0, len(lines)-((component.end-component.begin)+1)+len(replacement))
	out = append(out, lines[:component.begin]...)
	out = append(out, replacement...)
	out = append(out, lines[component.end+1:]...)
	return out
}

func replaceAllComponents(lines []string, replacements map[int][]string) []string {
	if len(replacements) == 0 {
		return lines
	}
	out := make([]string, 0, len(lines)+16)
	for i := 0; i < len(lines); {
		if replacement, ok := replacements[i]; ok {
			out = append(out, replacement...)
			depth := 0
			for i < len(lines) {
				name := propName(lines[i])
				value := strings.ToUpper(strings.TrimSpace(propValue(lines[i])))
				if name == "BEGIN" && value == "VEVENT" {
					depth++
				}
				if name == "END" && value == "VEVENT" {
					depth--
					if depth == 0 {
						i++
						break
					}
				}
				i++
			}
			continue
		}
		out = append(out, lines[i])
		i++
	}
	return out
}
