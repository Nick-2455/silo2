package synthesis

import "context"

type Source struct {
	Title       string
	Content     string
	ContextHint string
}

type Proposal struct {
	ProposedSummary  string   `json:"proposed_summary"`
	SuggestedThemes  []string `json:"suggested_themes"`
	WhyItMightMatter string   `json:"why_it_might_matter"`
}

type Synthesizer interface {
	Synthesize(context.Context, Source) (Proposal, error)
}
