package synthesis

import (
	"context"
	"reflect"
	"testing"
)

func TestNewConfigured_FallsBackDeterministically(t *testing.T) {
	src := Source{Title: "Title", Content: "Body"}
	expected, err := NewFallback().Synthesize(context.Background(), src)
	if err != nil {
		t.Fatalf("fallback synthesize: %v", err)
	}

	tests := []struct {
		name     string
		provider string
		model    string
		apiKey   string
	}{
		{name: "empty provider disables ai"},
		{name: "invalid provider falls back", provider: "made-up"},
		{name: "missing key falls back", provider: "openai", model: "gpt-4.1-mini"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewConfigured(tt.provider, tt.model, tt.apiKey).Synthesize(context.Background(), src)
			if err != nil {
				t.Fatalf("Synthesize: %v", err)
			}
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("proposal = %+v, want fallback %+v", got, expected)
			}
		})
	}
}
