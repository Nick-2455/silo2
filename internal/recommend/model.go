package recommend

// Recommendation represents a suggested activity.
type Recommendation struct {
	Title            string `json:"title"`
	Source           string `json:"source"` // seed filename or URL
	Type             string `json:"type"`   // video, article, book, podcast
	DurationEstimate int    `json:"duration_estimate"`
	Category         string `json:"category"` // ai-cup, exam, oracle-ios, personal
	Score            int    `json:"score"`
	Label            string `json:"label"` // watch-now, watch-later, expand, requires-prerequisite, skip
	Reason           string `json:"reason,omitempty"`
}

// SeedInput is the minimal representation of an open seed for scoring.
type SeedInput struct {
	Title         string
	Path          string
	Frontmatter   map[string]string
	EstimatedMins int
	Tags          []string
	CreatedAt     string // YYYY-MM-DD
}

// Engine produces ranked recommendations from available inputs.
type Engine interface {
	Recommend(profile Profile, seeds []SeedInput, freeMinutes int) ([]Recommendation, error)
	RecommendWithHints(profile Profile, seeds []SeedInput, freeMinutes int, hints Hints) ([]Recommendation, error)
}

// Hints carries optional recommendation context that can bias scoring.
type Hints struct {
	ProductiveHours [][2]string
}

// Profile holds the user's context for recommendation scoring.
type Profile struct {
	CurrentFocus  []string
	Interests     []string
	LearningGoals []string
}
