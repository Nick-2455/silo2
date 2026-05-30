package schedule

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedule.json")
	s := NewStore(path)

	sch := Schedule{
		Events: []ScheduleEvent{
			{
				ID:              "evt-1",
				Title:           "Test event",
				Type:            EventTypeFixed,
				Start:           "09:00",
				DurationMinutes: 30,
			},
		},
	}

	if err := s.Save(sch); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(loaded.Events))
	}
	if loaded.Events[0].Title != "Test event" {
		t.Fatalf("expected 'Test event', got %q", loaded.Events[0].Title)
	}
}

func TestStore_LoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent", "schedule.json")
	s := NewStore(path)

	sch, err := s.Load()
	if err != nil {
		t.Fatalf("Load should return empty schedule on missing file: %v", err)
	}
	if len(sch.Events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(sch.Events))
	}
}

func TestStore_AddEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedule.json")
	s := NewStore(path)

	ev := ScheduleEvent{
		Title:           "New event",
		Type:            EventTypeFixed,
		Start:           "10:00",
		DurationMinutes: 60,
	}

	added, err := s.AddEvent(ev)
	if err != nil {
		t.Fatalf("AddEvent: %v", err)
	}

	if added.ID == "" {
		t.Fatalf("expected generated ID, got empty")
	}
	if added.Title != "New event" {
		t.Fatalf("expected 'New event', got %q", added.Title)
	}

	// Verify persistence.
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Events) != 1 {
		t.Fatalf("expected 1 event after AddEvent, got %d", len(loaded.Events))
	}
}

func TestStore_RemoveEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedule.json")
	s := NewStore(path)

	ev, err := s.AddEvent(ScheduleEvent{
		Title:           "To remove",
		Type:            EventTypeFixed,
		Start:           "11:00",
		DurationMinutes: 30,
	})
	if err != nil {
		t.Fatalf("AddEvent: %v", err)
	}

	if err := s.RemoveEvent(ev.ID); err != nil {
		t.Fatalf("RemoveEvent: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Events) != 0 {
		t.Fatalf("expected 0 events after remove, got %d", len(loaded.Events))
	}
}

func TestStore_RemoveNonExistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedule.json")
	s := NewStore(path)

	err := s.RemoveEvent("nonexistent-id")
	if err == nil {
		t.Fatalf("expected error for nonexistent ID")
	}
}

func TestStore_ListEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schedule.json")
	s := NewStore(path)

	_, _ = s.AddEvent(ScheduleEvent{Title: "A", Start: "08:00", DurationMinutes: 30})
	_, _ = s.AddEvent(ScheduleEvent{Title: "B", Start: "09:00", DurationMinutes: 45})

	events, err := s.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestStore_SaveCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "deep", "schedule.json")
	s := NewStore(path)

	sch := Schedule{Events: []ScheduleEvent{}}
	if err := s.Save(sch); err != nil {
		t.Fatalf("Save should create parent dirs: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist: %v", err)
	}
}
