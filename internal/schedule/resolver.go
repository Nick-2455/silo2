package schedule

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

var weekdayMap = map[string]time.Weekday{
	"mon": time.Monday,
	"tue": time.Tuesday,
	"wed": time.Wednesday,
	"thu": time.Thursday,
	"fri": time.Friday,
	"sat": time.Saturday,
	"sun": time.Sunday,
}

// ResolveDay returns events applicable to the given date, sorted by start time.
func ResolveDay(sch Schedule, date string) ([]ResolvedEvent, error) {
	target, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("invalid date %q: %w", date, err)
	}

	var resolved []ResolvedEvent
	for _, ev := range sch.Events {
		if !appliesToDate(ev, target) {
			continue
		}
		start, end, err := resolveTimes(ev, date)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, ResolvedEvent{
			ScheduleEvent: ev,
			Date:          date,
			Start:         start,
			End:           end,
		})
	}

	sort.Slice(resolved, func(i, j int) bool {
		return resolved[i].Start < resolved[j].Start
	})
	return resolved, nil
}

// FreeSlots returns non-overlapping free slots between start and end on the given date.
func FreeSlots(sch Schedule, date, start, end string) ([]TimeSlot, error) {
	if err := validateHHMM(start); err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}
	if err := validateHHMM(end); err != nil {
		return nil, fmt.Errorf("invalid end time: %w", err)
	}
	if start >= end {
		return nil, fmt.Errorf("start time %q must be before end time %q", start, end)
	}

	events, err := ResolveDay(sch, date)
	if err != nil {
		return nil, err
	}

	// Filter events that overlap with the search window and convert to intervals.
	var busy []TimeSlot
	for _, ev := range events {
		if ev.Start < end && ev.End > start {
			busy = append(busy, TimeSlot{
				Start: max(ev.Start, start),
				End:   min(ev.End, end),
			})
		}
	}

	// Merge overlapping busy intervals.
	merged := mergeIntervals(busy)

	// Build free slots from gaps between merged busy intervals.
	var free []TimeSlot
	cursor := start
	for _, b := range merged {
		if b.Start > cursor {
			free = append(free, newTimeSlot(cursor, b.Start))
		}
		if b.End > cursor {
			cursor = b.End
		}
	}
	if cursor < end {
		free = append(free, newTimeSlot(cursor, end))
	}

	return free, nil
}

// ValidateEvent verifies that an event has valid timing fields before it is persisted.
func ValidateEvent(ev ScheduleEvent) error {
	if err := validateHHMM(ev.Start); err != nil {
		return fmt.Errorf("event %q has invalid start time: %w", ev.Title, err)
	}
	if ev.DurationMinutes <= 0 {
		return fmt.Errorf("event %q must have duration > 0", ev.Title)
	}
	return nil
}

func appliesToDate(ev ScheduleEvent, target time.Time) bool {
	if len(ev.Days) == 0 {
		return true // no days specified means every day
	}
	dateStr := target.Format("2006-01-02")
	weekday := strings.ToLower(target.Weekday().String())[:3]
	for _, d := range ev.Days {
		low := strings.ToLower(strings.TrimSpace(d))
		if low == dateStr {
			return true
		}
		if low == weekday {
			return true
		}
	}
	return false
}

func resolveTimes(ev ScheduleEvent, date string) (string, string, error) {
	if err := ValidateEvent(ev); err != nil {
		return "", "", err
	}

	startTime, err := time.Parse("15:04", ev.Start)
	if err != nil {
		return "", "", err
	}
	endTime := startTime.Add(time.Duration(ev.DurationMinutes) * time.Minute)
	startStr := startTime.Format("15:04")
	endStr := endTime.Format("15:04")
	return startStr, endStr, nil
}

func newTimeSlot(start, end string) TimeSlot {
	startTime, _ := time.Parse("15:04", start)
	endTime, _ := time.Parse("15:04", end)
	return TimeSlot{Start: start, End: end, DurationMinutes: int(endTime.Sub(startTime).Minutes())}
}

func validateHHMM(s string) error {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return fmt.Errorf("expected HH:MM, got %q", s)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid hour in %q", s)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid minute in %q", s)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return fmt.Errorf("time %q out of range", s)
	}
	return nil
}

func mergeIntervals(intervals []TimeSlot) []TimeSlot {
	if len(intervals) == 0 {
		return nil
	}

	// Sort by start time.
	sort.Slice(intervals, func(i, j int) bool {
		return intervals[i].Start < intervals[j].Start
	})

	merged := []TimeSlot{intervals[0]}
	for i := 1; i < len(intervals); i++ {
		last := &merged[len(merged)-1]
		if intervals[i].Start <= last.End {
			if intervals[i].End > last.End {
				last.End = intervals[i].End
			}
		} else {
			merged = append(merged, intervals[i])
		}
	}
	return merged
}
