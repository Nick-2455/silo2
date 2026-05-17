package synthesis

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"
)

const (
	maxThemes         = 8
	maxSummaryRunes   = 200
	maxWhyMatterRunes = 240
)

type proposalPayload struct {
	ProposedSummary  string   `json:"proposed_summary"`
	SuggestedThemes  []string `json:"suggested_themes"`
	WhyItMightMatter string   `json:"why_it_might_matter"`
}

func ParseProposal(raw string) (Proposal, error) {
	var payload proposalPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return Proposal{}, err
	}
	return normalizeProposal(Proposal{
		ProposedSummary:  payload.ProposedSummary,
		SuggestedThemes:  payload.SuggestedThemes,
		WhyItMightMatter: payload.WhyItMightMatter,
	})
}

func normalizeProposal(p Proposal) (Proposal, error) {
	p.ProposedSummary = clampRunes(normalizeWhitespace(p.ProposedSummary), maxSummaryRunes)
	p.WhyItMightMatter = clampRunes(normalizeWhitespace(p.WhyItMightMatter), maxWhyMatterRunes)
	p.SuggestedThemes = normalizeThemes(p.SuggestedThemes)

	if p.ProposedSummary == "" || p.WhyItMightMatter == "" || len(p.SuggestedThemes) == 0 {
		return Proposal{}, errors.New("proposal is incomplete")
	}
	return p, nil
}

func normalizeThemes(themes []string) []string {
	seen := make(map[string]struct{}, len(themes))
	normalized := make([]string, 0, min(len(themes), maxThemes))
	for _, theme := range themes {
		theme = normalizeWhitespace(theme)
		if theme == "" {
			continue
		}
		if _, ok := seen[theme]; ok {
			continue
		}
		seen[theme] = struct{}{}
		normalized = append(normalized, theme)
		if len(normalized) == maxThemes {
			break
		}
	}
	return normalized
}

func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func clampRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:limit]))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
