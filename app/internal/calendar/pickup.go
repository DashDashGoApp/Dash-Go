package calendar

import (
	"fmt"
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

func PickupEvents(dayKey, title, uid string, every int, start, end time.Time, holidays map[string]bool, values map[string]string) []Event {
	weekday := weekdayIndex(dayKey)
	if weekday < 0 {
		return nil
	}
	day := start
	for int(day.Weekday()+6)%7 != weekday {
		day = day.AddDate(0, 0, 1)
	}
	out := []Event{}
	for day.Before(end) {
		shifted := MaybeShiftPickup(day, holidays, values)
		out = append(out, Event{Date: shifted, Summary: title, UID: uid})
		day = day.AddDate(0, 0, 7*every)
	}
	return out
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

func MaybeShiftPickup(day time.Time, holidays map[string]bool, values map[string]string) time.Time {
	if values["PICKUP_HOLIDAY_SHIFT"] != "1" || len(holidays) == 0 {
		return day
	}
	days := atoiClamp(values["PICKUP_SHIFT_DAYS"], 1, 1, 7)
	weekStart := day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
	for raw := range holidays {
		holiday, err := time.Parse("20060102", raw)
		if err != nil {
			continue
		}
		if !holiday.Before(weekStart) && !holiday.After(day) {
			if strings.HasPrefix(strings.ToLower(values["PICKUP_SHIFT"]), "back") {
				return day.AddDate(0, 0, -days)
			}
			return day.AddDate(0, 0, days)
		}
	}
	return day
}

func PaydayEvents(values map[string]string, years []int, start, end time.Time) []Event {
	out := []Event{}
	mode := strings.ToLower(values["PAYDAY_MODE"])
	if mode == "weekly" || mode == "biweekly" {
		day, err := time.Parse("2006-01-02", values["PAYDAY_START"])
		if err != nil {
			return out
		}
		step := 7
		if mode == "biweekly" {
			step = 14
		}
		for day.Before(start) {
			day = day.AddDate(0, 0, step)
		}
		for day.Before(end) {
			out = append(out, Event{Date: day, Summary: "Payday", UID: "payday"})
			day = day.AddDate(0, 0, step)
		}
	} else if mode == "monthly" {
		day := atoiClamp(values["PAYDAY_DAY"], 1, 1, 28)
		for _, year := range years {
			for month := time.January; month <= time.December; month++ {
				out = append(out, Event{Date: DateOnly(year, month, day), Summary: "Payday", UID: "payday"})
			}
		}
	}
	return out
}

func CelebrationICSEvents(path string, years []int, start, end time.Time) []Event {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := []Event{}
	for index, raw := range strings.Split(string(body), "\n") {
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
			for _, year := range years {
				value := DateOnly(year, time.Month(month), day)
				if int(value.Month()) == month && value.Day() == day {
					out = append(out, Event{Date: value, Summary: label, UID: fmt.Sprintf("celebration-%d", index+1)})
				}
			}
		} else if reISODate.MatchString(datePart) {
			value, err := time.Parse("2006-01-02", datePart)
			if err == nil && !value.Before(start) && value.Before(end) {
				out = append(out, Event{Date: value, Summary: label, UID: fmt.Sprintf("special-%d", index+1)})
			}
		}
	}
	return out
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
