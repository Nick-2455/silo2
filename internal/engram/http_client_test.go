package engram

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// exportEnvelope mimics the exact JSON Engram returns from GET /export.
// Observations sit inside the envelope; sessions and prompts are present
// in real responses but we intentionally do not consume them.
const exportEnvelope = `{
  "version": "0.1.0",
  "exported_at": "2026-05-17 01:10:58",
  "sessions": [],
  "observations": [
    {
      "id": 321,
      "sync_id": "obs-a794b21d",
      "session_id": "manual-save-silo2",
      "type": "architecture",
      "title": "Silo2 MVP scaffold bootstrapped",
      "content": "body here",
      "project": "silo2",
      "scope": "project",
      "topic_key": "architecture/silo2-mvp",
      "revision_count": 1,
      "duplicate_count": 1,
      "last_seen_at": "2026-05-16 22:51:44",
      "created_at": "2026-05-16 22:51:44",
      "updated_at": "2026-05-16 22:51:44"
    },
    {
      "id": 200,
      "sync_id": "obs-99463c5f",
      "type": "decision",
      "title": "Renamed Marrow project to Silo",
      "content": "second body",
      "project": "silo2",
      "scope": "project",
      "created_at": "2026-05-14 10:38:55"
    }
  ],
  "prompts": []
}`

// searchArrayResponse is the shape returned by GET /search — a top-level
// JSON array of observations (with an extra `rank` we ignore).
const searchArrayResponse = `[
  {
    "id": 321,
    "sync_id": "obs-a794b21d",
    "type": "architecture",
    "title": "Silo2 MVP scaffold bootstrapped",
    "content": "body here",
    "project": "silo2",
    "created_at": "2026-05-16 22:51:44",
    "rank": -1.897e-06
  }
]`

func TestHTTPClient_Context_HitsExport(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/export" {
			t.Errorf("unexpected path: %s (Context must use /export, /observations is POST-only on real Engram)", r.URL.Path)
		}
		if got := r.URL.Query().Get("project"); got != "silo2" {
			t.Errorf("unexpected project: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(exportEnvelope))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "", 2*time.Second)
	got, err := c.Context(context.Background(), "silo2")
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 obs, got %d", len(got))
	}

	first := got[0]
	if first.ID != "321" {
		t.Errorf("expected ID=\"321\" (integer flattened to string), got %q", first.ID)
	}
	if first.Type != "architecture" {
		t.Errorf("unexpected type: %s", first.Type)
	}
	if first.Project != "silo2" {
		t.Errorf("unexpected project: %s", first.Project)
	}
	want := time.Date(2026, 5, 16, 22, 51, 44, 0, time.UTC)
	if !first.CreatedAt.Equal(want) {
		t.Errorf("CreatedAt = %v, want %v", first.CreatedAt, want)
	}
}

func TestHTTPClient_Context_EmptyProject_OmitsParam(t *testing.T) {
	// Empty/whitespace project must not send `project=` so /export returns
	// the full dump. The mock asserts the absence of the query param.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("expected no query params, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"observations":[]}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "", 2*time.Second)
	got, err := c.Context(context.Background(), "   ")
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d", len(got))
	}
}

func TestHTTPClient_Context_NullEnvelope_ReturnsEmpty(t *testing.T) {
	// Defensive: if /export ever returns `null` for an unknown project
	// instead of an empty envelope, do not crash — return [].
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("null"))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "", 2*time.Second)
	got, err := c.Context(context.Background(), "ghost")
	if err != nil {
		t.Fatalf("Context: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d", len(got))
	}
}

func TestHTTPClient_Search_PassesQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "silo" {
			t.Errorf("unexpected q: %s", got)
		}
		_, _ = w.Write([]byte(searchArrayResponse))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "", 2*time.Second)
	got, err := c.Search(context.Background(), "silo")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 obs, got %d", len(got))
	}
	if got[0].ID != "321" {
		t.Errorf("expected ID 321, got %q", got[0].ID)
	}
}

func TestHTTPClient_AuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("missing/wrong auth header: %q", got)
		}
		_, _ = w.Write([]byte(`{"observations":[]}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "test-key", 2*time.Second)
	if _, err := c.Context(context.Background(), "silo2"); err != nil {
		t.Fatalf("Context: %v", err)
	}
}

func TestHTTPClient_Non2xx_IncludesBody(t *testing.T) {
	// Real Engram returns 4xx/5xx with JSON {"error":"..."}; we want the
	// snippet surfaced in the Go error chain so failures are actionable.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":"search: SQL logic error"}`))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "", 2*time.Second)
	_, err := c.Search(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "SQL logic error") {
		t.Errorf("error should include status and body snippet: %v", err)
	}
}

func TestHTTPClient_Search_NullBody_ReturnsEmpty(t *testing.T) {
	// Engram returns the literal `null` when /search has zero matches.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("null"))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "", 2*time.Second)
	got, err := c.Search(context.Background(), "no-matches")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice for null body, got %d", len(got))
	}
}

func TestHTTPClient_BadJSON_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := NewHTTPClient(srv.URL, "", 2*time.Second)
	if _, err := c.Search(context.Background(), "x"); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestWireObservation_FallsBackToSyncID(t *testing.T) {
	w := wireObservation{SyncID: "obs-abc", Title: "x"}
	got := w.toObservation()
	if got.ID != "obs-abc" {
		t.Errorf("expected fallback to sync_id, got %q", got.ID)
	}
}
