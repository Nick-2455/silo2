package seed

import (
	"strings"
	"testing"
	"time"

	"github.com/nicolasperalta/silo2/internal/engram"
)

// The Seed primitive is the AI-generated synthesis proposal. These tests
// enforce the MVP contract independent of any model: deterministic IDs,
// stable themes, passthrough of human-provided Why, and zero hidden
// timestamps. If any of these break, the human triage loop breaks too.

func TestMockGenerator_DeterministicForSameInput(t *testing.T) {
	g := NewMockGenerator()
	obs := engram.Observation{
		ID:        "obs-123",
		Title:     "MVVM-C navigation insight",
		Content:   "Coordinators decouple navigation from view models in iOS apps.",
		Project:   "silo2",
		CreatedAt: time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC),
	}

	s1, err := g.Generate(obs)
	if err != nil {
		t.Fatalf("Generate (1): %v", err)
	}
	s2, err := g.Generate(obs)
	if err != nil {
		t.Fatalf("Generate (2): %v", err)
	}

	if s1.ID != s2.ID {
		t.Errorf("ID drifted across calls: %q vs %q", s1.ID, s2.ID)
	}
	if s1.Title != s2.Title {
		t.Errorf("Title drifted: %q vs %q", s1.Title, s2.Title)
	}
	if s1.ProposedSummary != s2.ProposedSummary {
		t.Errorf("ProposedSummary drifted")
	}
	if strings.Join(s1.SuggestedThemes, ",") != strings.Join(s2.SuggestedThemes, ",") {
		t.Errorf("Themes drifted: %v vs %v", s1.SuggestedThemes, s2.SuggestedThemes)
	}
}

func TestMockGenerator_DifferentContentSameObservationID_GivesDifferentSeedID(t *testing.T) {
	// In mock mode the backend reassigns the same "obs-mock-1" on every
	// process start. If seed IDs derived only from observation IDs, two
	// captures with different content in different sessions would
	// collapse onto the same seed file and silently lose the second one.
	// Pin the fix: the ID must reflect the content.
	g := NewMockGenerator()

	s1, _ := g.Generate(engram.Observation{ID: "obs-mock-1", Content: "first capture"})
	s2, _ := g.Generate(engram.Observation{ID: "obs-mock-1", Content: "second capture"})

	if s1.ID == s2.ID {
		t.Errorf("different content with same ID collided on seed ID: %s", s1.ID)
	}
}

func TestMockGenerator_SameContentDifferentObservationID_GivesDifferentSeedID(t *testing.T) {
	// Per-observation seeds should not collapse just because content is
	// identical. Two separate captures of the same text in two different
	// observations are still two distinct seeds.
	g := NewMockGenerator()

	s1, _ := g.Generate(engram.Observation{ID: "obs-a", Content: "same text"})
	s2, _ := g.Generate(engram.Observation{ID: "obs-b", Content: "same text"})

	if s1.ID == s2.ID {
		t.Errorf("different observation IDs collided on seed ID: %s", s1.ID)
	}
}

func TestMockGenerator_WhyChangesSeedID(t *testing.T) {
	// Why is part of the synthesis input. Adding or changing it must
	// produce a different seed so the human triage surface reflects the
	// new context, not the old one. (This keeps the human in the loop;
	// silently re-using the old seed would hide the changed intent.)
	g := NewMockGenerator()

	s1, _ := g.Generate(engram.Observation{ID: "obs-1", Content: "same"})
	s2, _ := g.Generate(engram.Observation{ID: "obs-1", Content: "same", Why: "now I added context"})

	if s1.ID == s2.ID {
		t.Errorf("Why was not factored into seed ID: %s", s1.ID)
	}
}

func TestMockGenerator_IDStableAcrossObservationOrder(t *testing.T) {
	// Seed IDs derive from sorted observation IDs so that re-saving the
	// same logical input never produces a duplicate seed in Inbox/open/.
	g := NewMockGenerator()

	o1 := engram.Observation{ID: "obs-a", Content: "x"}
	o2 := engram.Observation{ID: "obs-b", Content: "y"}

	// Order shouldn't matter for ID stability (we currently generate one
	// seed per observation, but the ID algorithm must already tolerate
	// multi-source seeds for future regeneration logic).
	s1, _ := g.Generate(o1)
	s2, _ := g.Generate(o2)

	if s1.ID == s2.ID {
		t.Errorf("different observations produced colliding seed IDs: %s", s1.ID)
	}
	if !strings.HasPrefix(s1.ID, "seed-") {
		t.Errorf("seed ID missing prefix: %q", s1.ID)
	}
	if len(s1.ID) != len("seed-")+8 {
		t.Errorf("seed ID unexpected length: %q", s1.ID)
	}
}

func TestMockGenerator_TitleFallback(t *testing.T) {
	g := NewMockGenerator()

	// Empty title: fall back to first words of content.
	s, err := g.Generate(engram.Observation{
		ID:      "obs-1",
		Title:   "",
		Content: "Coordinators decouple navigation from view models in iOS apps cleanly.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.Title == "" {
		t.Fatal("title fallback produced empty")
	}
	if strings.HasPrefix(s.Title, "Untitled") {
		t.Errorf("expected content-derived title, got %q", s.Title)
	}

	// Empty title AND empty content: fall back to literal sentinel.
	s2, err := g.Generate(engram.Observation{ID: "obs-2"})
	if err != nil {
		t.Fatal(err)
	}
	if s2.Title != "Untitled seed" {
		t.Errorf("expected 'Untitled seed', got %q", s2.Title)
	}
}

func TestMockGenerator_ThemesAreWeak(t *testing.T) {
	// MVP contract: the mock MUST NOT teach the system any taxonomy.
	// Themes always come back as ["unclassified"] until a real synthesis
	// model is introduced. Changing this default is a product decision,
	// not an implementation detail — keep the test loud.
	g := NewMockGenerator()

	for _, content := range []string{
		"architecture decision about coordinator pattern",
		"random text with no signal at all",
		"",
		"identity reflection on craft",
	} {
		s, err := g.Generate(engram.Observation{ID: "x", Content: content})
		if err != nil {
			t.Fatal(err)
		}
		if len(s.SuggestedThemes) != 1 || s.SuggestedThemes[0] != "unclassified" {
			t.Errorf("themes should be [unclassified] for content %q, got %v",
				content, s.SuggestedThemes)
		}
	}
}

func TestMockGenerator_PreservesUserWhy(t *testing.T) {
	// Why is the most valuable human signal in the system. The generator
	// must passthrough it verbatim — never reinterpret, never edit,
	// never merge into other fields.
	g := NewMockGenerator()

	s, err := g.Generate(engram.Observation{
		ID:      "obs-1",
		Title:   "MVVM-C insight",
		Content: "body",
		Why:     "I want to remember this approach for the next iOS project.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.UserWhy != "I want to remember this approach for the next iOS project." {
		t.Errorf("UserWhy not preserved: %q", s.UserWhy)
	}
	// Why must NOT leak into the AI-authored summary.
	if strings.Contains(s.ProposedSummary, "I want to remember") {
		t.Errorf("UserWhy leaked into ProposedSummary: %q", s.ProposedSummary)
	}
}

func TestMockGenerator_SummaryTruncates(t *testing.T) {
	g := NewMockGenerator()
	long := strings.Repeat("a", 500)
	s, err := g.Generate(engram.Observation{ID: "x", Content: long})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ProposedSummary) > 300 {
		// 200 chars + prefix + ellipsis ≈ well under 300.
		t.Errorf("ProposedSummary not truncated, len=%d", len(s.ProposedSummary))
	}
	if !strings.Contains(s.ProposedSummary, "[…]") {
		t.Errorf("expected ellipsis marker, got %q", s.ProposedSummary)
	}
}

func TestMockGenerator_LinksSourceObservation(t *testing.T) {
	g := NewMockGenerator()
	s, err := g.Generate(engram.Observation{ID: "obs-42", Content: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.SourceObservationIDs) != 1 || s.SourceObservationIDs[0] != "obs-42" {
		t.Errorf("source observation not linked: %v", s.SourceObservationIDs)
	}
}
