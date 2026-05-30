package schedule

import (
	"testing"
	"time"
)

func TestValidateEvent_ValidFixed(t *testing.T) {
	ev := ScheduleEvent{
		Title:           "Daily standup",
		Type:            EventTypeFixed,
		Start:           "09:00",
		DurationMinutes: 30,
	}
	if err := ValidateEvent(ev); err != nil {
		t.Fatalf("expected valid event, got: %v", err)
	}
}

func TestValidateEvent_InvalidStartTime(t *testing.T) {
	tests := []struct {
		name  string
		start string
	}{
		{"empty start", ""},
		{"no colon", "0900"},
		{"invalid hour", "25:00"},
		{"invalid minute", "09:60"},
		{"out of range", "24:01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := ScheduleEvent{
				Title:           "Bad event",
				Start:           tt.start,
				DurationMinutes: 30,
			}
			if err := ValidateEvent(ev); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestValidateEvent_ZeroDuration(t *testing.T) {
	ev := ScheduleEvent{
		Title:           "Zero minutes",
		Start:           "09:00",
		DurationMinutes: 0,
	}
	if err := ValidateEvent(ev); err == nil {
		t.Fatalf("expected error for zero duration, got nil")
	}
}

func TestResolveDay_EmptySchedule(t *testing.T) {
	sch := Schedule{Events: []ScheduleEvent{}}
	events, err := ResolveDay(sch, "2026-05-30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestResolveDay_InvalidDate(t *testing.T) {
	sch := Schedule{Events: []ScheduleEvent{}}
	_, err := ResolveDay(sch, "not-a-date")
	if err == nil {
		t.Fatalf("expected error for invalid date")
	}
}

func TestResolveDay_FixedEventOnExactDate(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	sch := Schedule{
		Events: []ScheduleEvent{
			{
				ID:              "evt-1",
				Title:           "Meeting",
				Type:            EventTypeFixed,
				Start:           "10:00",
				DurationMinutes: 60,
				Days:            []string{today},
			},
		},
	}
	events, err := ResolveDay(sch, today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Title != "Meeting" {
		t.Fatalf("expected Meeting, got %s", events[0].Title)
	}
	if events[0].Start != "10:00" || events[0].End != "11:00" {
		t.Fatalf("expected 10:00-11:00, got %s-%s", events[0].Start, events[0].End)
	}
}

func TestResolveDay_EveryDayEvent(t *testing.T) {
	sch := Schedule{
		Events: []ScheduleEvent{
			{
				ID:              "evt-1",
				Title:           "Daily routine",
				Type:            EventTypeRoutine,
				Start:           "08:00",
				DurationMinutes: 15,
				Days:            nil, // every day
			},
		},
	}
	events, err := ResolveDay(sch, "2026-05-30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestResolveDay_RoutineByWeekday(t *testing.T) {
	// 2026-05-30 is a Saturday.
	sch := Schedule{
		Events: []ScheduleEvent{
			{
				ID:              "evt-1",
				Title:           "Monday meeting",
				Type:            EventTypeRoutine,
				Start:           "09:00",
				DurationMinutes: 30,
				Days:            []string{"mon"},
			},
			{
				ID:              "evt-2",
				Title:           "Saturday class",
				Type:            EventTypeRoutine,
				Start:           "10:00",
				DurationMinutes: 120,
				Days:            []string{"sat"},
			},
		},
	}
	events, err := ResolveDay(sch, "2026-05-30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event (Saturday class), got %d", len(events))
	}
	if events[0].Title != "Saturday class" {
		t.Fatalf("expected Saturday class, got %s", events[0].Title)
	}
}

func TestFreeSlots_NoEvents(t *testing.T) {
	sch := Schedule{Events: []ScheduleEvent{}}
	slots, err := FreeSlots(sch, "2026-05-30", "06:00", "22:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slots) != 1 {
		t.Fatalf("expected 1 free slot, got %d", len(slots))
	}
	if slots[0].Start != "06:00" || slots[0].End != "22:00" {
		t.Fatalf("expected 06:00-22:00, got %s-%s", slots[0].Start, slots[0].End)
	}
	if slots[0].DurationMinutes != 960 { // 16h * 60
		t.Fatalf("expected 960 min, got %d", slots[0].DurationMinutes)
	}
}

func TestFreeSlots_WithBusyEvent(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	sch := Schedule{
		Events: []ScheduleEvent{
			{
				ID:              "evt-1",
				Title:           "Busy block",
				Type:            EventTypeFixed,
				Start:           "10:00",
				DurationMinutes: 120, // 10:00-12:00
				Days:            []string{today},
			},
		},
	}
	slots, err := FreeSlots(sch, today, "06:00", "22:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect two slots: 06:00-10:00 and 12:00-22:00
	if len(slots) != 2 {
		t.Fatalf("expected 2 free slots, got %d", len(slots))
	}
	if slots[0].Start != "06:00" || slots[0].End != "10:00" {
		t.Fatalf("first slot: expected 06:00-10:00, got %s-%s", slots[0].Start, slots[0].End)
	}
	if slots[1].Start != "12:00" || slots[1].End != "22:00" {
		t.Fatalf("second slot: expected 12:00-22:00, got %s-%s", slots[1].Start, slots[1].End)
	}
}

func TestFreeSlots_EventOutsideWindow(t *testing.T) {
	today := time.Now().Format("2006-01-02")
	sch := Schedule{
		Events: []ScheduleEvent{
			{
				ID:              "evt-1",
				Title:           "Early meeting",
				Type:            EventTypeFixed,
				Start:           "05:00",
				DurationMinutes: 60,
				Days:            []string{today},
			},
		},
	}
	slots, err := FreeSlots(sch, today, "08:00", "18:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Event at 05:00-06:00 does not overlap with 08:00-18:00 window.
	if len(slots) != 1 {
		t.Fatalf("expected 1 free slot, got %d", len(slots))
	}
	if slots[0].Start != "08:00" || slots[0].End != "18:00" {
		t.Fatalf("expected 08:00-18:00, got %s-%s", slots[0].Start, slots[0].End)
	}
}

func TestFreeSlots_InvalidStartTime(t *testing.T) {
	sch := Schedule{Events: []ScheduleEvent{}}
	_, err := FreeSlots(sch, "2026-05-30", "25:00", "22:00")
	if err == nil {
		t.Fatalf("expected error for invalid start time")
	}
}

func TestFreeSlots_StartAfterEnd(t *testing.T) {
	sch := Schedule{Events: []ScheduleEvent{}}
	_, err := FreeSlots(sch, "2026-05-30", "18:00", "08:00")
	if err == nil {
		t.Fatalf("expected error for start after end")
	}
}
