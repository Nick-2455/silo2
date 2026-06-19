package recommend

import (
	"reflect"
	"testing"
)

func TestSimpleEngine_RecommendEmpty(t *testing.T) {
	e := NewEngine()
	recs, err := e.Recommend(
		Profile{CurrentFocus: []string{"Go"}},
		nil,
		120,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations, got %d", len(recs))
	}
}

func TestSimpleEngine_ProfileMatchScoresHigher(t *testing.T) {
	e := NewEngine()

	seeds := []SeedInput{
		{Title: "Go Generics Deep Dive", EstimatedMins: 30, Tags: []string{"go", "programming"}},
		{Title: "Cooking Pasta from Scratch", EstimatedMins: 45, Tags: []string{"cooking"}},
	}

	recs, err := e.Recommend(
		Profile{CurrentFocus: []string{"Go", "Architecture"}},
		seeds,
		120,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(recs))
	}
	if recs[0].Score <= recs[1].Score {
		t.Fatalf("Go seed should score higher: Go=%d, Cooking=%d", recs[0].Score, recs[1].Score)
	}
	if recs[0].Title != "Go Generics Deep Dive" {
		t.Fatalf("expected Go seed first, got %s", recs[0].Title)
	}
}

func TestSimpleEngine_DurationFit(t *testing.T) {
	e := NewEngine()

	seeds := []SeedInput{
		{Title: "Short article", EstimatedMins: 15, Tags: []string{"go"}},
		{Title: "Long video", EstimatedMins: 200, Tags: []string{"go"}},
	}

	recs, err := e.Recommend(
		Profile{CurrentFocus: []string{"Go"}},
		seeds,
		60, // only 60 minutes free
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(recs) != 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(recs))
	}

	// Short article should score higher because it fits in 60 min.
	if recs[0].Score <= recs[1].Score {
		t.Fatalf("short article should score higher: short=%d, long=%d", recs[0].Score, recs[1].Score)
	}
}

func TestSimpleEngine_TopFive(t *testing.T) {
	e := NewEngine()

	seeds := make([]SeedInput, 10)
	for i := range seeds {
		seeds[i] = SeedInput{
			Title:         "Seed",
			EstimatedMins: 10,
		}
	}

	recs, err := e.Recommend(Profile{}, seeds, 120)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 5 {
		t.Fatalf("expected 5 (top 5), got %d", len(recs))
	}
}

func TestSimpleEngine_RecommendWithHints_MatchesRecommendWhenHintsEmpty(t *testing.T) {
	t.Parallel()

	e := NewEngine()
	profile := Profile{CurrentFocus: []string{"Go", "Architecture"}}
	seeds := []SeedInput{
		{Title: "Go architecture deep dive", EstimatedMins: 30, Tags: []string{"go", "architecture"}},
		{Title: "Gardening basics", EstimatedMins: 15, Tags: []string{"garden"}},
	}

	base, err := e.Recommend(profile, seeds, 60)
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}

	withHints, err := e.RecommendWithHints(profile, seeds, 60, Hints{})
	if err != nil {
		t.Fatalf("RecommendWithHints() error = %v", err)
	}

	if !reflect.DeepEqual(withHints, base) {
		t.Fatalf("RecommendWithHints() = %#v, want %#v", withHints, base)
	}
}

func TestSimpleEngine_RecommendWithHints_IsDeterministic(t *testing.T) {
	t.Parallel()

	e := NewEngine()
	profile := Profile{CurrentFocus: []string{"Go"}}
	seeds := []SeedInput{
		{Title: "Go testing patterns", EstimatedMins: 25, Tags: []string{"go", "testing"}},
		{Title: "System design", EstimatedMins: 45, Tags: []string{"architecture"}},
	}
	hints := Hints{ProductiveHours: [][2]string{{"08:00", "12:00"}}}

	first, err := e.RecommendWithHints(profile, seeds, 90, hints)
	if err != nil {
		t.Fatalf("first RecommendWithHints() error = %v", err)
	}
	second, err := e.RecommendWithHints(profile, seeds, 90, hints)
	if err != nil {
		t.Fatalf("second RecommendWithHints() error = %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("RecommendWithHints() first = %#v, second = %#v", first, second)
	}
}

func TestSimpleEngine_RecommendWithHints_AcceptsProductiveHours(t *testing.T) {
	t.Parallel()

	e := NewEngine()
	_, err := e.RecommendWithHints(
		Profile{CurrentFocus: []string{"Go"}},
		[]SeedInput{{Title: "Go testing", EstimatedMins: 20, Tags: []string{"go"}}},
		60,
		Hints{ProductiveHours: [][2]string{{"08:00", "12:00"}, {"14:00", "18:00"}}},
	)
	if err != nil {
		t.Fatalf("RecommendWithHints() error = %v", err)
	}
}

func TestSimpleEngine_Classification(t *testing.T) {
	tests := []struct {
		score int
		label string
	}{
		{60, "watch-now"},
		{55, "watch-now"},
		{45, "watch-later"},
		{40, "watch-later"},
		{30, "expand"},
		{25, "expand"},
		{10, "skip"},
		{0, "skip"},
	}
	for _, tt := range tests {
		if got := classify(tt.score); got != tt.label {
			t.Errorf("classify(%d) = %q, want %q", tt.score, got, tt.label)
		}
	}
}

func TestRenderMarkdown_GroupsByLabel(t *testing.T) {
	recs := []Recommendation{
		{Title: "High priority", Label: "watch-now", Score: 60, Reason: "matches focus"},
		{Title: "Medium", Label: "watch-later", Score: 40, Reason: "interesting"},
		{Title: "Low", Label: "skip", Score: 5, Reason: "not relevant"},
	}

	md := RenderMarkdown("2026-06-01", 120, recs)

	if len(md) == 0 {
		t.Fatal("expected non-empty markdown")
	}
	if !contains(md, "Ver ahora") {
		t.Error("expected 'Ver ahora' section")
	}
	if !contains(md, "Ver después") {
		t.Error("expected 'Ver después' section")
	}
	if !contains(md, "High priority") {
		t.Error("expected 'High priority' in output")
	}
}

func contains(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && len(s) >= len(sub) && searchSubstring(s, sub)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
