package setup

// InferredBlock represents a routine block extracted from natural language.
type InferredBlock struct {
	Title           string   // e.g. "Despertar", "Trabajo", "Almuerzo"
	Start           string   // HH:MM
	DurationMinutes int
	Category        string   // wake_up, work, study, lunch, exercise, dinner, sleep, hobby
	Days            []string // weekday keys or empty for all days
	DaysSpecified   bool     // true when user explicitly stated days in original answer
	Confirmed       bool     // true after user confirms
}

// InterviewState holds the full interview context.
type InterviewState struct {
	Blocks          []InferredBlock
	ProductiveHours [][2]string // pairs of [start, end] in HH:MM
	Answers         []string    // raw user answers
}
