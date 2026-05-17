package synthesis

import (
	"context"
	"reflect"
	"testing"
)

func TestParseProposal_IgnoresUnknownFields(t *testing.T) {
	raw := `{
		"proposed_summary": "  Proposal summary.  ",
		"suggested_themes": [" architecture ", "", "architecture", "testing"],
		"why_it_might_matter": "  Helps later. ",
		"extra": "noise",
		"internal_meta": {"source": "test"},
		"ignored_field": ["unused"]
	}`

	got, err := ParseProposal(raw)
	if err != nil {
		t.Fatalf("ParseProposal() error = %v", err)
	}

	want := Proposal{
		ProposedSummary:  "Proposal summary.",
		SuggestedThemes:  []string{"architecture", "testing"},
		WhyItMightMatter: "Helps later.",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseProposal() mismatch\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestParseProposal_NormalizesThemesAndRejectsInvalidPayloads(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    Proposal
		wantErr bool
	}{
		{
			name: "dedupes trims and caps themes",
			raw: `{
				"proposed_summary": "summary",
				"suggested_themes": [" one ", "two", "one", "three", "four", "five", "six", "seven", "eight", "nine"],
				"why_it_might_matter": "matters"
			}`,
			want: Proposal{
				ProposedSummary:  "summary",
				SuggestedThemes:  []string{"one", "two", "three", "four", "five", "six", "seven", "eight"},
				WhyItMightMatter: "matters",
			},
		},
		{
			name:    "rejects missing proposal field",
			raw:     `{"proposed_summary":"summary","suggested_themes":["one"]}`,
			wantErr: true,
		},
		{
			name:    "rejects bad json",
			raw:     `{"proposed_summary":`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseProposal(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseProposal() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseProposal() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseProposal() mismatch\nwant: %#v\ngot:  %#v", tt.want, got)
			}
		})
	}
}

func TestNewSynthesizer_UsesFallbackWhenProviderMissing(t *testing.T) {
	src := Source{Title: "Useful note", Content: "A body worth summarizing.", ContextHint: "captured during review"}

	got, err := New(nil, "").Synthesize(context.Background(), src)
	if err != nil {
		t.Fatalf("Synthesize() error = %v", err)
	}

	want, err := NewFallback().Synthesize(context.Background(), src)
	if err != nil {
		t.Fatalf("fallback Synthesize() error = %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("New(nil, \"\") should use fallback\nwant: %#v\ngot:  %#v", want, got)
	}
}
