package engram

import "time"

type Observation struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Project string `json:"project"`

	// TopicKey is Engram's stable per-topic identifier (e.g.
	// "architecture/engram-http-api"). Optional; empty when the source
	// observation has none. Used by the Curated layer to group multiple
	// observations of the same topic into one human-editable note.
	TopicKey string `json:"topic_key,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// IdentitySignal is a future-friendly placeholder for signals extracted from observations.
// In the MVP we keep it minimal and derive identity directly from raw observations.
type IdentitySignal struct {
	Kind   string
	Value  string
	Source string
}
