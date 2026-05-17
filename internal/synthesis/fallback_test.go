package synthesis

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestFallbackSynthesize_OnlyPopulatesProposalFieldsDeterministically(t *testing.T) {
	tests := []struct {
		name string
		src  Source
	}{
		{
			name: "observation content",
			src: Source{
				Title:       " Layered architecture insight ",
				Content:     " Layered architecture helps keep domain logic independent from infrastructure details. ",
				ContextHint: "captured from a project note",
			},
		},
		{
			name: "empty content fallback",
			src:  Source{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewFallback()

			got1, err := s.Synthesize(context.Background(), tt.src)
			if err != nil {
				t.Fatalf("Synthesize() error = %v", err)
			}
			got2, err := s.Synthesize(context.Background(), tt.src)
			if err != nil {
				t.Fatalf("Synthesize() second call error = %v", err)
			}

			if !reflect.DeepEqual(got1, got2) {
				t.Fatalf("fallback should be deterministic\nfirst:  %#v\nsecond: %#v", got1, got2)
			}
			if strings.TrimSpace(got1.ProposedSummary) == "" {
				t.Fatal("ProposedSummary must be populated")
			}
			if len(got1.SuggestedThemes) == 0 {
				t.Fatal("SuggestedThemes must be populated")
			}
			if strings.TrimSpace(got1.WhyItMightMatter) == "" {
				t.Fatal("WhyItMightMatter must be populated")
			}
		})
	}
}

func TestFallbackSynthesize_NormalizesThemesAndSummary(t *testing.T) {
	long := strings.Repeat("a", 500)

	got, err := NewFallback().Synthesize(context.Background(), Source{
		Title:       "  Testing fallback  ",
		Content:     long,
		ContextHint: "review later",
	})
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	if len(got.SuggestedThemes) == 0 || len(got.SuggestedThemes) > maxThemes {
		t.Fatalf("SuggestedThemes length = %d, want 1..%d", len(got.SuggestedThemes), maxThemes)
	}
	for _, theme := range got.SuggestedThemes {
		if strings.TrimSpace(theme) == "" {
			t.Fatalf("theme must not be empty: %#v", got.SuggestedThemes)
		}
	}
	if len([]rune(got.ProposedSummary)) > maxSummaryRunes {
		t.Fatalf("ProposedSummary too long: %d", len([]rune(got.ProposedSummary)))
	}
	if strings.Contains(got.ProposedSummary, "  ") {
		t.Fatalf("ProposedSummary should be normalized, got %q", got.ProposedSummary)
	}
}
