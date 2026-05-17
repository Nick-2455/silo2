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

	// Why is optional capture context provided by the human at save time
	// (e.g. via `silo save --why "..."`). It is metadata about WHY this
	// observation was captured, not a reinterpretation of its content.
	//
	// Memory is sacred: Why MUST NOT be merged into Content. Synthesis
	// layers may surface Why separately and attribute it to the human.
	//
	// Engram persistence note: as of Engram v1.15.13 there is no `why`
	// column in the observation schema. HTTPClient.Save forwards the
	// field in the POST payload (so a future Engram version with native
	// `why` support starts roundtripping it without a client change),
	// but the value is silently dropped today. The durable record of
	// Why lives in the seed file's "Capture Why" section.
	Why string `json:"why,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// IdentitySignal is a future-friendly placeholder for signals extracted from observations.
// In the MVP we keep it minimal and derive identity directly from raw observations.
type IdentitySignal struct {
	Kind   string
	Value  string
	Source string
}
