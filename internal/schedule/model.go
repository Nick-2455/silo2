package schedule

// EventType defines the kind of schedule event.
type EventType string

const (
	EventTypeFixed   EventType = "fixed"
	EventTypeRoutine EventType = "routine"
)

// ScheduleEvent represents a single event in the schedule.
type ScheduleEvent struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Type            EventType `json:"type"`
	Start           string    `json:"start"` // HH:MM
	DurationMinutes int       `json:"duration_minutes"`
	Days            []string  `json:"days"` // weekday keys or YYYY-MM-DD
	Category        string    `json:"category,omitempty"`
}

// Schedule holds all events.
type Schedule struct {
	Version string          `json:"version"`
	Events  []ScheduleEvent `json:"events"`
}

// TimeSlot represents a free or busy interval.
type TimeSlot struct {
	Start           string `json:"start"` // HH:MM
	End             string `json:"end"`   // HH:MM
	DurationMinutes int    `json:"duration_minutes"`
}

// ResolvedEvent is a ScheduleEvent anchored to a specific date.
type ResolvedEvent struct {
	ScheduleEvent
	Date  string `json:"date"`  // YYYY-MM-DD
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`   // HH:MM
}
