package synthesis

import (
	"context"
	"strings"
)

const (
	fallbackTheme        = "unclassified"
	fallbackEmptyBody    = "(no content captured)"
	fallbackWhyItMatters = "Useful starting point for human review. Confirm what matters, rewrite what does not, and keep ownership of the final note."
	fallbackUntitled     = "Untitled note"
)

type fallbackSynthesizer struct{}

func NewFallback() Synthesizer {
	return fallbackSynthesizer{}
}

func (fallbackSynthesizer) Synthesize(_ context.Context, src Source) (Proposal, error) {
	summary := normalizeWhitespace(src.Content)
	if summary == "" {
		summary = fallbackEmptyBody
	}
	if summary != fallbackEmptyBody {
		summary = clampRunes(summary, maxSummaryRunes)
	}

	theme := normalizeWhitespace(src.Title)
	if theme == "" {
		theme = fallbackTheme
	} else {
		theme = strings.ToLower(strings.Join(strings.Fields(theme), "-"))
	}

	return normalizeProposal(Proposal{
		ProposedSummary:  summary,
		SuggestedThemes:  []string{theme, fallbackTheme},
		WhyItMightMatter: fallbackWhyItMatters,
	})
}
