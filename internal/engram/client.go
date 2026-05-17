package engram

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nicolasperalta/silo2/internal/config"
)

// Client is the contract every Engram backend must implement.
// All callers (sync, profile, future enrichments) must depend on this
// interface, never on a concrete type. This keeps mock and real HTTP
// implementations fully interchangeable.
type Client interface {
	Search(ctx context.Context, query string) ([]Observation, error)
	Context(ctx context.Context, project string) ([]Observation, error)

	// Save persists an observation and returns the resulting Engram ID.
	// Memory layer contract: Save is the ONLY write path. Observations
	// are append-only; nothing in this codebase ever mutates or deletes
	// a saved observation.
	//
	// Implementations may reject the call (e.g. HTTP backend during the
	// MVP) by returning ErrSaveUnsupported, in which case the caller is
	// expected to surface a friendly hint rather than crashing.
	Save(ctx context.Context, obs Observation) (string, error)
}

// ErrSaveUnsupported is the sentinel returned by backends that do not yet
// implement Save. Callers may use errors.Is to detect it and offer a
// targeted message instead of a generic failure.
var ErrSaveUnsupported = errors.New("engram backend does not support Save")

// NewClient is the only construction path. It routes to MockClient when no
// endpoint is configured, otherwise returns a real HTTPClient.
func NewClient(cfg *config.Config) Client {
	if cfg == nil || strings.TrimSpace(cfg.EngramEndpoint) == "" {
		return NewMockClient()
	}
	return NewHTTPClient(cfg.EngramEndpoint, cfg.EngramAPIKey, 10*time.Second)
}

// MockClient returns hardcoded observations. Used when engram_endpoint is
// empty, so the MVP flow works fully offline.
//
// Save appends to the in-memory slice with a generated ID. The mutex
// keeps concurrent saves safe even though the CLI is single-threaded —
// tests sometimes drive the mock from goroutines.
type MockClient struct {
	mu           sync.Mutex
	observations []Observation
	saveCount    int
}

func NewMockClient() *MockClient {
	return &MockClient{
		observations: []Observation{
			{
				ID:        "obs-001",
				Title:     "Silo: Engram is the source of truth",
				Type:      "architecture",
				Content:   "Silo projects knowledge from Engram into Markdown notes for Obsidian. Silo must not create another memory store.",
				Project:   "silo2",
				TopicKey:  "architecture/silo-source-of-truth",
				CreatedAt: time.Date(2026, 5, 16, 14, 10, 0, 0, time.UTC),
			},
			{
				ID:        "obs-002",
				Title:     "Identity: Nicolas Peralta profile seed",
				Type:      "identity",
				Content:   "Nicolas Peralta is a software architect focused on developer tooling and knowledge management. Primary language: Go. Also ships SwiftUI macOS/iOS apps.",
				Project:   "silo2",
				CreatedAt: time.Date(2026, 5, 16, 14, 15, 0, 0, time.UTC),
			},
			{
				ID:        "obs-003",
				Title:     "Project: Engram integration plan",
				Type:      "decision",
				Content:   "Start with a mock Engram client and keep an HTTP client stub. Avoid third-party dependencies in MVP.",
				Project:   "silo2",
				CreatedAt: time.Date(2026, 5, 16, 14, 20, 0, 0, time.UTC),
			},
			{
				ID:        "obs-004",
				Title:     "Skills snapshot",
				Type:      "learning",
				Content:   "Skills: Go, architecture, clean design, SwiftUI. Interests: local-first tools, knowledge graphs, developer experience.",
				Project:   "silo2",
				CreatedAt: time.Date(2026, 5, 16, 14, 25, 0, 0, time.UTC),
			},
			// Edge case: empty title -> filename must fall back to observation-<id>.
			{
				ID:        "obs-005",
				Title:     "",
				Type:      "note",
				Content:   "Untitled thought captured on the fly.",
				Project:   "silo2",
				CreatedAt: time.Date(2026, 5, 16, 14, 30, 0, 0, time.UTC),
			},
			// Edge case: weird characters in title -> slug must sanitize.
			{
				ID:        "obs-006",
				Title:     "Engram & Silo: notes / drafts!",
				Type:      "note",
				Content:   "Punctuation soup. The slug should be safe.",
				Project:   "silo2",
				CreatedAt: time.Date(2026, 5, 16, 14, 35, 0, 0, time.UTC),
			},
			// Edge case: would collide with obs-008 after slugging.
			{
				ID:        "obs-007",
				Title:     "Silo Design",
				Type:      "decision",
				Content:   "First design draft for Silo.",
				Project:   "silo2",
				TopicKey:  "architecture/silo-design",
				CreatedAt: time.Date(2026, 5, 16, 14, 40, 0, 0, time.UTC),
			},
			{
				ID:        "obs-008",
				Title:     "  SILO   design  ",
				Type:      "decision",
				Content:   "Same topic, different casing/spacing. Forces collision resolution and topic_key grouping in curated layer.",
				Project:   "silo2",
				TopicKey:  "architecture/silo-design",
				CreatedAt: time.Date(2026, 5, 16, 14, 45, 0, 0, time.UTC),
			},
		},
	}
}

func (m *MockClient) Search(_ context.Context, query string) ([]Observation, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return append([]Observation(nil), m.observations...), nil
	}
	var out []Observation
	for _, o := range m.observations {
		if strings.Contains(strings.ToLower(o.Title), q) || strings.Contains(strings.ToLower(o.Content), q) {
			out = append(out, o)
		}
	}
	return out, nil
}

func (m *MockClient) Context(_ context.Context, project string) ([]Observation, error) {
	p := strings.ToLower(strings.TrimSpace(project))
	if p == "" {
		return append([]Observation(nil), m.observations...), nil
	}
	var out []Observation
	for _, o := range m.observations {
		if strings.ToLower(o.Project) == p {
			out = append(out, o)
		}
	}
	return out, nil
}

// Save appends an observation to the in-memory store and returns a
// generated mock ID ("obs-mock-N"). The provided obs.ID is ignored — the
// store assigns IDs to keep the Memory layer's "Engram owns identity"
// rule in place even for the mock backend.
func (m *MockClient) Save(_ context.Context, obs Observation) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveCount++
	obs.ID = fmt.Sprintf("obs-mock-%d", m.saveCount)
	if obs.CreatedAt.IsZero() {
		obs.CreatedAt = time.Now().UTC()
	}
	m.observations = append(m.observations, obs)
	return obs.ID, nil
}

// HTTPClient.Save lives in http_save.go; this file owns the interface
// and the in-process mock backend only.

// Compile-time guarantee that *http.Client satisfies the standard library
// http Doer-like usage we rely on in http_client.go. Kept here as a tiny
// reminder that NewClient is the only sanctioned construction path.
var _ = (*http.Client)(nil)
