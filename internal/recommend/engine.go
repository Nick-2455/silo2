package recommend

import (
	"sort"
	"strings"
)

// SimpleEngine is the v1 recommendation engine.
// It scores seeds based on profile matching, time fit, and recency.
type SimpleEngine struct{}

// NewEngine returns a SimpleEngine.
func NewEngine() *SimpleEngine {
	return &SimpleEngine{}
}

// Recommend scores and ranks seeds, returning top 5.
func (e *SimpleEngine) Recommend(profile Profile, seeds []SeedInput, freeMinutes int) ([]Recommendation, error) {
	var recs []Recommendation

	for _, s := range seeds {
		score := 10 // base score

		// Profile match: check if seed tags/title match current focus.
		if matchProfile(profile, s) {
			score += 30
		}

		// Duration fit: prefer seeds that fit within free time.
		if s.EstimatedMins > 0 && s.EstimatedMins <= freeMinutes {
			score += 20
		} else if s.EstimatedMins > freeMinutes {
			score -= 10
		}

		// Recency: newer seeds score slightly higher.
		// Not implemented in v1 — all seeds get same recency bonus.
		score += 5

		label := classify(score)
		category := inferCategory(s)

		recs = append(recs, Recommendation{
			Title:            s.Title,
			Source:           s.Path,
			Type:             seedType(s),
			DurationEstimate: s.EstimatedMins,
			Category:         category,
			Score:            score,
			Label:            label,
			Reason:           reasonForLabel(label, profile, s, freeMinutes),
		})
	}

	// Sort by score descending.
	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Score > recs[j].Score
	})

	// Return top 5.
	if len(recs) > 5 {
		recs = recs[:5]
	}

	return recs, nil
}

// RecommendWithHints keeps the base engine deterministic while allowing
// additive scoring signals to be introduced without changing the original API.
func (e *SimpleEngine) RecommendWithHints(profile Profile, seeds []SeedInput, freeMinutes int, hints Hints) ([]Recommendation, error) {
	if len(hints.ProductiveHours) == 0 {
		return e.Recommend(profile, seeds, freeMinutes)
	}

	recs, err := e.Recommend(profile, seeds, freeMinutes)
	if err != nil {
		return nil, err
	}

	for i := range recs {
		if recs[i].DurationEstimate > 0 && recs[i].DurationEstimate <= freeMinutes {
			recs[i].Score++
		}
	}

	sort.Slice(recs, func(i, j int) bool {
		if recs[i].Score == recs[j].Score {
			return recs[i].Title < recs[j].Title
		}
		return recs[i].Score > recs[j].Score
	})

	return recs, nil
}

func matchProfile(p Profile, s SeedInput) bool {
	focus := strings.ToLower(strings.Join(p.CurrentFocus, " "))
	title := strings.ToLower(s.Title)

	// Check if any focus keyword appears in title or tags.
	for _, f := range p.CurrentFocus {
		if strings.Contains(title, strings.ToLower(f)) {
			return true
		}
	}
	for _, tag := range s.Tags {
		if strings.Contains(focus, strings.ToLower(tag)) {
			return true
		}
	}
	return false
}

func classify(score int) string {
	switch {
	case score >= 55:
		return "watch-now"
	case score >= 40:
		return "watch-later"
	case score >= 25:
		return "expand"
	default:
		return "skip"
	}
}

func inferCategory(s SeedInput) string {
	tags := strings.ToLower(strings.Join(s.Tags, " "))
	for _, cat := range []string{"ai-cup", "exam", "oracle-ios"} {
		if strings.Contains(tags, cat) {
			return cat
		}
	}
	return "personal"
}

func seedType(s SeedInput) string {
	t, ok := s.Frontmatter["type"]
	if ok {
		return t
	}
	return "article"
}

func reasonForLabel(label string, p Profile, s SeedInput, freeMin int) string {
	switch label {
	case "watch-now":
		return "matches your current focus and fits your free time"
	case "watch-later":
		return "relevant but needs more time or lower priority"
	case "expand":
		return "could be worth exploring when you have more context"
	default:
		return "not recommended right now"
	}
}
