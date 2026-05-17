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

func TestNewConfigured_OpenCode(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
	}{
		{name: "lowercase", provider: "opencode", model: "anthropic/claude-haiku-4-5"},
		{name: "uppercase", provider: "OPENCODE", model: "model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewConfigured(tt.provider, tt.model, "")
			synth, ok := got.(providerSynthesizer)
			if !ok {
				t.Fatalf("NewConfigured() = %T, want providerSynthesizer", got)
			}
			if synth.provider == nil {
				t.Fatal("provider = nil, want opencode provider")
			}
		})
	}
}

func TestNewConfigured_OpenCodeEmptyModel_Fallback(t *testing.T) {
	got := NewConfigured("opencode", "", "")
	if _, ok := got.(fallbackSynthesizer); !ok {
		t.Fatalf("NewConfigured() = %T, want fallbackSynthesizer", got)
	}
}

func TestNewConfigured_OpenAIUnchanged(t *testing.T) {
	if got := NewConfigured("openai", "gpt-4.1-mini", ""); !reflect.DeepEqual(reflect.TypeOf(got), reflect.TypeOf(NewFallback())) {
		t.Fatalf("NewConfigured(openai without key) = %T, want fallback", got)
	}
	if got := NewConfigured("openai", "gpt-4.1-mini", "test-key"); reflect.TypeOf(got) != reflect.TypeOf(providerSynthesizer{}) {
		t.Fatalf("NewConfigured(openai with key) = %T, want providerSynthesizer", got)
	}
}
