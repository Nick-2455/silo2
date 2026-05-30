package schedule

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Store persists a Schedule as JSON.
type Store struct {
	path string
}

// NewStore creates a Store at the given file path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Load reads the schedule from disk. If the file does not exist, returns an empty schedule.
func (s *Store) Load() (Schedule, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Schedule{Events: []ScheduleEvent{}}, nil
		}
		return Schedule{}, fmt.Errorf("schedule: read %s: %w", s.path, err)
	}

	var sch Schedule
	if err := json.Unmarshal(data, &sch); err != nil {
		return Schedule{}, fmt.Errorf("schedule: parse %s: %w", s.path, err)
	}
	return sch, nil
}

// Save writes the schedule to disk, creating parent directories as needed.
func (s *Store) Save(sch Schedule) error {
	data, err := json.MarshalIndent(sch, "", "  ")
	if err != nil {
		return fmt.Errorf("schedule: marshal: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("schedule: mkdir %s: %w", dir, err)
	}

	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return fmt.Errorf("schedule: write %s: %w", s.path, err)
	}
	return nil
}

// AddEvent appends a new event with a generated ID and saves the schedule.
func (s *Store) AddEvent(ev ScheduleEvent) (ScheduleEvent, error) {
	sch, err := s.Load()
	if err != nil {
		return ScheduleEvent{}, err
	}

	if ev.ID == "" {
		ev.ID = generateID()
	}
	sch.Events = append(sch.Events, ev)

	if err := s.Save(sch); err != nil {
		return ScheduleEvent{}, err
	}
	return ev, nil
}

// RemoveEvent deletes the event with the given ID and saves the schedule.
func (s *Store) RemoveEvent(id string) error {
	sch, err := s.Load()
	if err != nil {
		return err
	}

	found := false
	filtered := make([]ScheduleEvent, 0, len(sch.Events))
	for _, ev := range sch.Events {
		if ev.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, ev)
	}
	if !found {
		return fmt.Errorf("schedule: event not found: %s", id)
	}

	sch.Events = filtered
	return s.Save(sch)
}

// ListEvents returns all events in the schedule.
func (s *Store) ListEvents() ([]ScheduleEvent, error) {
	sch, err := s.Load()
	if err != nil {
		return nil, err
	}
	return sch.Events, nil
}

func generateID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("evt-%d", os.Getpid())
	}
	return hex.EncodeToString(b)
}
