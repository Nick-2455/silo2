package seed

import (
	"strings"
	"testing"
)

// The renderer is the bridge between the in-memory Seed and the file on
// disk a human will read in Obsidian. Its single most important property
// is determinism: same Seed → byte-identical markdown, every run.
// Without that, WriteNoteIfAbsent cannot protect human edits.

func TestRender_DeterministicForSameSeed(t *testing.T) {
	s := Seed{
		ID:                   "seed-abc12345",
		Title:                "MVVM-C navigation insight",
		SourceObservationIDs: []string{"obs-123"},
		ProposedSummary:      "Coordinators decouple navigation from view models.",
		SuggestedThemes:      []string{"unclassified"},
		WhyItMightMatter:     "Open this seed and decide.",
		UserWhy:              "I want to remember this for the next iOS project.",
	}

	out1, err := Render(s)
	if err != nil {
		t.Fatalf("Render (1): %v", err)
	}
	out2, err := Render(s)
	if err != nil {
		t.Fatalf("Render (2): %v", err)
	}
	if out1 != out2 {
		t.Fatal("renderer is not deterministic across calls")
	}
}

func TestRender_HasRequiredFrontmatter(t *testing.T) {
	s := Seed{
		ID:                   "seed-abc12345",
		Title:                "T",
		SourceObservationIDs: []string{"obs-123"},
		SuggestedThemes:      []string{"unclassified"},
	}
	out, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	// Frontmatter must come first, with the four MVP fields.
	for _, fragment := range []string{
		"---\ntype: seed\n",
		"status: open\n",
		"generated_by: silo\n",
		"source_observation: obs-123\n",
	} {
		if !strings.Contains(out, fragment) {
			t.Errorf("missing frontmatter fragment %q in:\n%s", fragment, out)
		}
	}
}

func TestRender_OmitsUserWhySectionWhenEmpty(t *testing.T) {
	// When no --why was passed, the rendered seed must not invent or
	// hint at a human reason. The section is dropped entirely.
	s := Seed{
		ID:                   "seed-1",
		Title:                "T",
		SourceObservationIDs: []string{"obs-1"},
		SuggestedThemes:      []string{"unclassified"},
	}
	out, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "Capture Why") {
		t.Errorf("Capture Why section appeared with empty UserWhy:\n%s", out)
	}
}

func TestRender_IncludesUserWhyAttributedToHuman(t *testing.T) {
	// When --why was provided, it MUST appear under a clearly human-
	// authored section, never inside an AI-authored section. This is the
	// proveniencia rule the user insisted on.
	s := Seed{
		ID:                   "seed-1",
		Title:                "T",
		SourceObservationIDs: []string{"obs-1"},
		ProposedSummary:      "AI summary text.",
		SuggestedThemes:      []string{"unclassified"},
		UserWhy:              "Because I keep forgetting this pattern.",
	}
	out, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Capture Why") {
		t.Errorf("missing Capture Why section:\n%s", out)
	}
	if !strings.Contains(out, "Because I keep forgetting this pattern.") {
		t.Errorf("UserWhy text not preserved:\n%s", out)
	}
	// Must not show up inside the AI summary.
	summaryIdx := strings.Index(out, "## Proposed Summary")
	whyIdx := strings.Index(out, "## Capture Why")
	if summaryIdx < 0 || whyIdx < 0 {
		t.Fatalf("section anchors missing")
	}
	// And the user text must not be sandwiched inside the summary block.
	summaryBlock := out[summaryIdx:whyIdx]
	if strings.Contains(summaryBlock, "Because I keep forgetting") {
		t.Errorf("UserWhy leaked into AI-authored Proposed Summary")
	}
}

func TestRender_HasHumanNotesPlaceholder(t *testing.T) {
	// The Human Notes section is the editorial gate. It must always be
	// present so the human knows where to write.
	s := Seed{
		ID: "seed-1", Title: "T",
		SourceObservationIDs: []string{"obs-1"},
		SuggestedThemes:      []string{"unclassified"},
	}
	out, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Human Notes") {
		t.Errorf("missing Human Notes section:\n%s", out)
	}
	if !strings.Contains(out, "TODO") {
		t.Errorf("expected TODO placeholder under Human Notes:\n%s", out)
	}
}

func TestRender_NoTimestamps(t *testing.T) {
	// Any timestamp generated at render time would break idempotency and
	// silently churn the file. Memory carries time; the synthesis layer
	// does not.
	s := Seed{
		ID: "seed-1", Title: "T",
		SourceObservationIDs: []string{"obs-1"},
		SuggestedThemes:      []string{"unclassified"},
	}
	out, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	// Quick heuristic: no 4-digit-year anywhere in the body.
	for _, year := range []string{"2024", "2025", "2026", "2027"} {
		if strings.Contains(out, year) {
			t.Errorf("renderer leaked a timestamp containing %q:\n%s", year, out)
		}
	}
}

func TestRender_MultipleSourcesListed(t *testing.T) {
	s := Seed{
		ID:                   "seed-1",
		Title:                "T",
		SourceObservationIDs: []string{"obs-a", "obs-b"},
		SuggestedThemes:      []string{"unclassified"},
	}
	out, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	// For multi-source seeds, frontmatter source_observation uses a list.
	if !strings.Contains(out, "source_observation:") {
		t.Errorf("missing source_observation field:\n%s", out)
	}
	if !strings.Contains(out, "obs-a") || !strings.Contains(out, "obs-b") {
		t.Errorf("multi-source IDs not both present:\n%s", out)
	}
}

func TestRender_IncludesLegacySourceSectionWhenPresent(t *testing.T) {
	s := Seed{
		ID:                   "seed-1",
		Title:                "T",
		SourceObservationIDs: []string{"obs-1"},
		SuggestedThemes:      []string{"unclassified"},
		LegacyPath:           "wiki/a/note.md",
	}
	out, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Source") {
		t.Fatalf("missing Source section:\n%s", out)
	}
	if !strings.Contains(out, "Legacy path: wiki/a/note.md") {
		t.Fatalf("missing legacy path line:\n%s", out)
	}
}

func TestRender_IncludesCallerOwnedSourceSectionBeforeHumanSections(t *testing.T) {
	s := Seed{
		ID:                   "seed-1",
		Title:                "T",
		SourceObservationIDs: []string{"obs-1"},
		SuggestedThemes:      []string{"unclassified"},
		SourceType:           "article",
		SourceURL:            "https://example.com/post",
		SourceTitle:          "Post title",
		UserWhy:              "Human reason",
	}

	out, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Sources\n\n- article: https://example.com/post") {
		t.Fatalf("missing caller-owned source section:\n%s", out)
	}
	sourcesIdx := strings.Index(out, "## Sources")
	whyIdx := strings.Index(out, "## Capture Why")
	notesIdx := strings.Index(out, "## Human Notes")
	if sourcesIdx < 0 || whyIdx < 0 || notesIdx < 0 {
		t.Fatalf("expected sections missing:\n%s", out)
	}
	if !(sourcesIdx < whyIdx && sourcesIdx < notesIdx) {
		t.Fatalf("Sources section must appear before human sections:\n%s", out)
	}
}

func TestRender_OmitsCallerOwnedSourceSectionWhenEmpty(t *testing.T) {
	s := Seed{
		ID:                   "seed-1",
		Title:                "T",
		SourceObservationIDs: []string{"obs-1"},
		SuggestedThemes:      []string{"unclassified"},
	}

	out, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "## Sources") {
		t.Fatalf("unexpected caller-owned source section:\n%s", out)
	}
}

func TestRender_IncludesSourceMetadataSectionWhenPresent(t *testing.T) {
	s := Seed{
		ID:                    "seed-1",
		Title:                 "T",
		SourceObservationIDs:  []string{"obs-1"},
		SuggestedThemes:       []string{"unclassified"},
		SourceType:            "video",
		SourceURL:             "https://example.com/watch?v=1",
		SourceTitle:           "Some video",
		SourceChannel:         "A channel",
		SourceDurationSeconds: 83,
	}
	out, err := Render(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Source Metadata") {
		t.Fatalf("missing Source Metadata section:\n%s", out)
	}
	if !strings.Contains(out, "Title: Some video") {
		t.Fatalf("missing title metadata:\n%s", out)
	}
	if !strings.Contains(out, "Channel: A channel") {
		t.Fatalf("missing channel metadata:\n%s", out)
	}
	if !strings.Contains(out, "Duration: 1:23") {
		t.Fatalf("missing duration metadata:\n%s", out)
	}
}

func TestRender_ErrorsOnInvalidSeed(t *testing.T) {
	if _, err := Render(Seed{}); err == nil {
		t.Error("expected error on zero-value seed (missing ID and sources)")
	}
}
