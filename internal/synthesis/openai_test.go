package synthesis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestOpenAIProviderComplete_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %s, want /v1/chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer test-key")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := body["model"]; got != defaultOpenAIModel {
			t.Fatalf("model = %v, want %q", got, defaultOpenAIModel)
		}

		_, _ = w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"content": "  {\"proposed_summary\":\"Short summary\",\"suggested_themes\":[\"testing\",\"go\"],\"why_it_might_matter\":\"Useful context\"}  "
					}
				}
			]
		}`))
	}))
	t.Cleanup(server.Close)

	provider := newOpenAIProvider("test-key", server.Client(), server.URL)
	raw, err := provider.Complete(context.Background(), Request{
		Source: Source{Title: "Title", Content: "Body"},
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	proposal, err := ParseProposal(raw)
	if err != nil {
		t.Fatalf("ParseProposal() error = %v", err)
	}
	if proposal.ProposedSummary != "Short summary" {
		t.Fatalf("ProposedSummary = %q, want %q", proposal.ProposedSummary, "Short summary")
	}
}

func TestOpenAIProviderComplete_MissingAPIKeySkipsNetwork(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	provider := newOpenAIProvider("", server.Client(), server.URL)
	_, err := provider.Complete(context.Background(), Request{Source: Source{Title: "Title", Content: "Body"}})
	if err == nil {
		t.Fatal("Complete() error = nil, want error")
	}
	if calls.Load() != 0 {
		t.Fatalf("server calls = %d, want 0", calls.Load())
	}
}

func TestOpenAIProviderComplete_HTTPAndPayloadErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   string
		wantErr    string
	}{
		{
			name:       "rate limited",
			statusCode: http.StatusTooManyRequests,
			response:   `{"error":{"message":"too many requests"}}`,
			wantErr:    "429",
		},
		{
			name:       "server error",
			statusCode: http.StatusBadGateway,
			response:   `{"error":{"message":"bad gateway"}}`,
			wantErr:    "502",
		},
		{
			name:       "bad json response",
			statusCode: http.StatusOK,
			response:   `{"choices":`,
			wantErr:    "decode",
		},
		{
			name:       "missing content",
			statusCode: http.StatusOK,
			response:   `{"choices":[{"message":{"content":""}}]}`,
			wantErr:    "content",
		},
		{
			name:       "non json content",
			statusCode: http.StatusOK,
			response:   `{"choices":[{"message":{"content":"not json"}}]}`,
			wantErr:    "json object",
		},
		{
			name:       "non object json content",
			statusCode: http.StatusOK,
			response:   `{"choices":[{"message":{"content":"[1,2,3]"}}]}`,
			wantErr:    "json object",
		},
		{
			name:       "quoted json string content",
			statusCode: http.StatusOK,
			response:   `{"choices":[{"message":{"content":"\"{\\\"proposed_summary\\\":\\\"summary\\\"}\""}}]}`,
			wantErr:    "json object",
		},
		{
			name:       "markdown wrapped json content",
			statusCode: http.StatusOK,
			response:   "{\"choices\":[{\"message\":{\"content\":\"```json\\n{\\\"proposed_summary\\\":\\\"summary\\\"}\\n```\"}}]}",
			wantErr:    "json object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.response))
			}))
			t.Cleanup(server.Close)

			provider := newOpenAIProvider("test-key", server.Client(), server.URL)
			_, err := provider.Complete(context.Background(), Request{Source: Source{Title: "Title", Content: "Body"}})
			if err == nil {
				t.Fatal("Complete() error = nil, want error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), tt.wantErr) {
				t.Fatalf("Complete() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestOpenAIProviderComplete_TimeoutOrCancel(t *testing.T) {
	tests := []struct {
		name   string
		client *http.Client
		ctx    func() (context.Context, context.CancelFunc)
	}{
		{
			name:   "context canceled",
			client: http.DefaultClient,
			ctx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
		},
		{
			name:   "client timeout",
			client: &http.Client{Timeout: 20 * time.Millisecond},
			ctx: func() (context.Context, context.CancelFunc) {
				return context.Background(), func() {}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(100 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
			}))
			t.Cleanup(server.Close)

			provider := newOpenAIProvider("test-key", tt.client, server.URL)
			ctx, cancel := tt.ctx()
			defer cancel()

			_, err := provider.Complete(ctx, Request{Source: Source{Title: "Title", Content: "Body"}})
			if err == nil {
				t.Fatal("Complete() error = nil, want error")
			}
		})
	}
}
