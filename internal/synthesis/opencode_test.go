package synthesis

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

const validOpenCodeProposal = `{"proposed_summary":"x","suggested_themes":["a"],"why_it_might_matter":"y"}`

func TestOpenCodeComplete_ExtractsLastTextPart(t *testing.T) {
	provider := newOpenCodeProviderWithRunner("opencode", func(context.Context, string, []string) ([]byte, error) {
		return []byte(strings.Join([]string{
			`{"type":"step_start"}`,
			openCodeTextLine("```json\n" + validOpenCodeProposal + "\n```"),
			`{"type":"step_finish"}`,
		}, "\n")), nil
	})

	raw, err := provider.Complete(context.Background(), Request{Model: "anthropic/claude-haiku-4-5", Source: Source{Title: "Title", Content: "Body"}})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if raw != validOpenCodeProposal {
		t.Fatalf("Complete() = %q, want %q", raw, validOpenCodeProposal)
	}
	if _, err := ParseProposal(raw); err != nil {
		t.Fatalf("ParseProposal() error = %v", err)
	}
}

func TestOpenCodeComplete_StripsMarkdownFences(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "json tag", text: "```json\n" + validOpenCodeProposal + "\n```"},
		{name: "no language tag", text: "```\n" + validOpenCodeProposal + "\n```"},
		{name: "raw json", text: validOpenCodeProposal},
		{name: "extra whitespace", text: " \n\t```json\n" + validOpenCodeProposal + "\n```\n "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newOpenCodeProviderWithRunner("opencode", fakeOpenCodeTextRunner(tt.text))

			raw, err := provider.Complete(context.Background(), Request{Model: "model", Source: Source{Title: "Title", Content: "Body"}})
			if err != nil {
				t.Fatalf("Complete() error = %v", err)
			}
			if raw != validOpenCodeProposal {
				t.Fatalf("Complete() = %q, want %q", raw, validOpenCodeProposal)
			}
		})
	}
}

func TestOpenCodeComplete_RejectsNonJSONObjectContent(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "plain text", text: "plain text not json"},
		{name: "json string", text: `"just a string"`},
		{name: "json array", text: `[1,2,3]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newOpenCodeProviderWithRunner("opencode", fakeOpenCodeTextRunner(tt.text))

			_, err := provider.Complete(context.Background(), Request{Model: "model", Source: Source{Title: "Title", Content: "Body"}})
			if err == nil {
				t.Fatal("Complete() error = nil, want error")
			}
		})
	}
}

func TestOpenCodeComplete_ErrorEvent(t *testing.T) {
	provider := newOpenCodeProviderWithRunner("opencode", func(context.Context, string, []string) ([]byte, error) {
		return []byte(`{"type":"error","error":{"data":{"message":"Insufficient balance"}}}`), nil
	})

	_, err := provider.Complete(context.Background(), Request{Model: "model", Source: Source{Title: "Title", Content: "Body"}})
	if err == nil {
		t.Fatal("Complete() error = nil, want error")
	}
	errorText := strings.ToLower(err.Error())
	for _, forbidden := range []string{"auth", "token", "key", "header"} {
		if strings.Contains(errorText, forbidden) {
			t.Fatalf("Complete() error = %q, must not contain %q", err.Error(), forbidden)
		}
	}
}

func TestOpenCodeComplete_CommandNotFound(t *testing.T) {
	provider := newOpenCodeProviderWithRunner("missing-opencode", func(context.Context, string, []string) ([]byte, error) {
		return nil, exec.ErrNotFound
	})

	_, err := provider.Complete(context.Background(), Request{Model: "model", Source: Source{Title: "Title", Content: "Body"}})
	if err == nil {
		t.Fatal("Complete() error = nil, want error")
	}
}

func TestOpenCodeComplete_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := newOpenCodeProviderWithRunner("opencode", func(ctx context.Context, _ string, _ []string) ([]byte, error) {
		return nil, ctx.Err()
	})

	_, err := provider.Complete(ctx, Request{Model: "model", Source: Source{Title: "Title", Content: "Body"}})
	if err == nil {
		t.Fatal("Complete() error = nil, want error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Complete() error = %v, want context.Canceled", err)
	}
}

func TestOpenCodeComplete_EmptyModel(t *testing.T) {
	called := false
	provider := newOpenCodeProviderWithRunner("opencode", func(context.Context, string, []string) ([]byte, error) {
		called = true
		return []byte{}, nil
	})

	_, err := provider.Complete(context.Background(), Request{Model: "", Source: Source{Title: "Title", Content: "Body"}})
	if err == nil {
		t.Fatal("Complete() error = nil, want error")
	}
	if called {
		t.Fatal("runner was called for empty model")
	}
}

func TestOpenCodeComplete_NoTextParts(t *testing.T) {
	provider := newOpenCodeProviderWithRunner("opencode", func(context.Context, string, []string) ([]byte, error) {
		return []byte(strings.Join([]string{`{"type":"step_start"}`, `{"type":"step_finish"}`}, "\n")), nil
	})

	_, err := provider.Complete(context.Background(), Request{Model: "model", Source: Source{Title: "Title", Content: "Body"}})
	if err == nil {
		t.Fatal("Complete() error = nil, want error")
	}
}

func TestOpenCodeComplete_MalformedNDJSONLineIgnored(t *testing.T) {
	provider := newOpenCodeProviderWithRunner("opencode", func(context.Context, string, []string) ([]byte, error) {
		return []byte("not json\n" + openCodeTextLine(validOpenCodeProposal)), nil
	})

	raw, err := provider.Complete(context.Background(), Request{Model: "model", Source: Source{Title: "Title", Content: "Body"}})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if raw != validOpenCodeProposal {
		t.Fatalf("Complete() = %q, want %q", raw, validOpenCodeProposal)
	}
}

func TestOpenCodeComplete_NeverLogsCommandArgs(t *testing.T) {
	// Secret hygiene review note: opencode.go must not log or print command args.
	// This test covers returned errors: they must not echo prompt text or model names.
	promptMarker := "prompt-marker-should-not-leak"
	model := "provider/model-should-not-leak"
	provider := newOpenCodeProviderWithRunner("opencode", func(context.Context, string, []string) ([]byte, error) {
		return nil, errors.New("runner failed")
	})

	_, err := provider.Complete(context.Background(), Request{Model: model, Source: Source{Title: promptMarker, Content: "Body"}})
	if err == nil {
		t.Fatal("Complete() error = nil, want error")
	}
	for _, forbidden := range []string{promptMarker, model} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("Complete() error = %q, must not contain %q", err.Error(), forbidden)
		}
	}
}

func fakeOpenCodeTextRunner(text string) opencodeRunner {
	return func(context.Context, string, []string) ([]byte, error) {
		return []byte(openCodeTextLine(text)), nil
	}
}

func openCodeTextLine(text string) string {
	line, err := json.Marshal(struct {
		Type string `json:"type"`
		Part struct {
			Text string `json:"text"`
		} `json:"part"`
	}{
		Type: "text",
		Part: struct {
			Text string `json:"text"`
		}{Text: text},
	})
	if err != nil {
		panic(err)
	}
	return string(line)
}
