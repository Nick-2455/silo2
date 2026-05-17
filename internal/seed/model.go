// Package seed implements the AI-generated synthesis proposal layer that
// sits between raw Engram observations and human-curated notes.
//
// Layer ownership (read this before changing anything here):
//
//   - Memory layer (Engram): append-only truth. Untouched by this package.
//   - Synthesis layer (this package): proposals only. Mutable, regenerable,
//     discardable. NEVER identity truth.
//   - Identity layer (Curated/): human-owned canon. Promotion is a human
//     act, expressed today as the human editing the seed's frontmatter
//     `status:` field or moving the file out of Inbox/open/.
//
// The package deliberately holds no LLM dependency. The Generator interface
// exists so a real model can be slotted in later without touching the rest
// of the workflow (capture, rendering, inbox listing).
package seed

// Seed is one synthesis proposal attached to one or more observations.
//
// Invariants:
//   - ID is deterministic from SourceObservationIDs (sorted). Re-generating
//     the same logical input never produces a duplicate seed on disk.
//   - SuggestedThemes are WEAK signals. The MVP generator always returns
//     ["unclassified"]; real generators may infer more, but the renderer
//     and inbox MUST treat any value as a proposal, never as taxonomy.
//   - UserWhy is verbatim human input. It is never modified, summarized,
//     or merged into AI-authored fields.
//
// A Seed is NOT identity truth. It exists to be reviewed.
type Seed struct {
	// ID is "seed-" + first 8 hex chars of sha256(sorted source IDs).
	ID string

	// Title is a short heading for the seed. Derived from the first
	// observation's title, with content/fallback rules in the generator.
	Title string

	// SourceObservationIDs are the Engram observation IDs this seed
	// synthesizes. Currently always length 1; the slice is here to keep
	// the contract open to multi-source seeds without future breakage.
	SourceObservationIDs []string

	// ProposedSummary is the AI's tentative reading of the observation.
	// MUST be presented to the user as a proposal, not a fact.
	ProposedSummary string

	// SuggestedThemes are weak signals. See package doc and generator.
	SuggestedThemes []string

	// WhyItMightMatter is the AI's contextualization. Same caveat as
	// ProposedSummary: it is a prompt for the human, not a verdict.
	WhyItMightMatter string

	// UserWhy is the verbatim --why text the human passed at capture
	// time, or empty. This is the single most valuable human signal in
	// the system; never edit it.
	UserWhy string
}
