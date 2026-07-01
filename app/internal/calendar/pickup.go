package calendar

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reMonthDay = regexp.MustCompile(`^\d{2}-\d{2}$`)
	reISODate  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

func LocalTimezoneName() string {
	if out, err := execOutput("timedatectl", "show", "-p", "Timezone", "--value"); err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out)
	}
	if body, err := os.ReadFile("/etc/timezone"); err == nil && strings.TrimSpace(string(body)) != "" {
		return strings.TrimSpace(string(body))
	}
	return "UTC"
}
func execOutput(name string, args ...string) (string, error) {
	body, err := exec.Command(name, args...).Output()
	return string(body), err
}

func weekdayIndex(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "mon", "monday":
		return 0
	case "tue", "tuesday":
		return 1
	case "wed", "wednesday":
		return 2
	case "thu", "thursday":
		return 3
	case "fri", "friday":
		return 4
	case "sat", "saturday":
		return 5
	case "sun", "sunday":
		return 6
	default:
		return -1
	}
}

// CelebrationICSEvents converts the small local celebration file into stable
// all-day events. February 29 entries are observed on February 28 in
// non-leap years so a family birthday never silently disappears.
func CelebrationICSEvents(path string, years []int, start, end time.Time) []Event {
	events, _ := celebrationICSEvents(path, years, start, end)
	return events
}

func celebrationICSEvents(path string, years []int, start, end time.Time) ([]Event, []string) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	out, notes := []Event{}, []string{}
	for _, raw := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "|") {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		datePart, label := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if label == "" {
			continue
		}
		if reMonthDay.MatchString(datePart) {
			month, _ := strconv.Atoi(datePart[:2])
			day, _ := strconv.Atoi(datePart[3:5])
			if !validCelebrationMonthDay(month, day) {
				continue
			}
			for _, year := range years {
				value, description := DateOnly(year, time.Month(month), day), ""
				if month == int(time.February) && day == 29 && !isLeapYear(year) {
					value = DateOnly(year, time.February, 28)
					description = "Observed February 28 in this non-leap year."
					notes = append(notes, label+" (02-29) observed February 28 in "+strconv.Itoa(year))
				}
				if !value.Before(start) && value.Before(end) {
					out = append(out, Event{Date: value, Summary: label, Description: description, UID: celebrationUID("celebration", datePart, label)})
				}
			}
		} else if reISODate.MatchString(datePart) {
			value, err := time.Parse("2006-01-02", datePart)
			if err == nil && !value.Before(start) && value.Before(end) {
				out = append(out, Event{Date: value, Summary: label, UID: celebrationUID("special", datePart, label)})
			}
		}
	}
	return out, notes
}

func validCelebrationMonthDay(month, day int) bool {
	if month < int(time.January) || month > int(time.December) || day < 1 {
		return false
	}
	value := DateOnly(2000, time.Month(month), day)
	return int(value.Month()) == month && value.Day() == day
}

func isLeapYear(year int) bool { return year%4 == 0 && (year%100 != 0 || year%400 == 0) }

func celebrationUID(kind, datePart, label string) string {
	sum := sha256.Sum256([]byte(kind + "|" + strings.TrimSpace(datePart) + "|" + strings.TrimSpace(label)))
	return kind + "-" + hex.EncodeToString(sum[:8])
}

func atoiClamp(value string, fallback, low, high int) int {
	number, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		number = fallback
	}
	if number < low {
		return low
	}
	if number > high {
		return high
	}
	return number
}
