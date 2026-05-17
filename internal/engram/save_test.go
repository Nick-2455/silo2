package engram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Save is the only write path into Memory. These tests pin down the two
// MVP guarantees: the mock backend appends and returns a generated ID,
// and the HTTP backend refuses cleanly via ErrSaveUnsupported so the
// CLI can show a helpful hint instead of crashing.

func TestMockClient_Save_AppendsAndAssignsID(t *testing.T) {
	m := NewMockClient()
	before := len(m.observations)

	id, err := m.Save(context.Background(), Observation{
		Title:   "captured",
		Content: "body",
		Project: "silo2",
		Why:     "because reasons",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !strings.HasPrefix(id, "obs-mock-") {
		t.Errorf("expected obs-mock-* ID, got %q", id)
	}
	if got := len(m.observations); got != before+1 {
		t.Errorf("expected %d observations after save, got %d", before+1, got)
	}

	saved := m.observations[len(m.observations)-1]
	if saved.ID != id {
		t.Errorf("stored ID %q != returned ID %q", saved.ID, id)
	}
	if saved.Why != "because reasons" {
		t.Errorf("Why not persisted: %q", saved.Why)
	}
	if saved.CreatedAt.IsZero() {
		t.Errorf("CreatedAt was not stamped at save time")
	}
}

func TestMockClient_Save_OverridesProvidedID(t *testing.T) {
	// Memory owns identity. Even if a caller passes an ID, the backend
	// must assign its own so a single source of truth holds.
	m := NewMockClient()
	id, err := m.Save(context.Background(), Observation{ID: "user-supplied", Content: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if id == "user-supplied" {
		t.Errorf("backend honored a user-supplied ID; should always generate")
	}
}

// HTTPClient.Save tests pin the wire contract against real Engram
// v1.15.13 (verified empirically against `engram serve`). Every test
// uses httptest so the suite runs offline.

func TestHTTPClient_Save_FullRoundtrip(t *testing.T) {
	// Two requests must hit Engram in order:
	//   1. POST /sessions (upsert) so the FK constraint on session_id holds
	//   2. POST /observations with the full payload
	// The mock asserts both call shapes and returns the real response bodies.
	var sessionCalled, observationCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)

		switch r.URL.Path {
		case "/sessions":
			sessionCalled = true
			var got map[string]any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("sessions body not JSON: %v", err)
			}
			if got["id"] != "silo-save-silo2" {
				t.Errorf("session id = %v, want silo-save-silo2", got["id"])
			}
			if got["project"] != "silo2" {
				t.Errorf("session project = %v", got["project"])
			}
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"id":"silo-save-silo2","status":"created"}`))

		case "/observations":
			observationCalled = true
			if !sessionCalled {
				t.Error("observation POST happened before session POST")
			}
			var got map[string]any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("observations body not JSON: %v", err)
			}
			if got["session_id"] != "silo-save-silo2" {
				t.Errorf("session_id = %v", got["session_id"])
			}
			if got["project"] != "silo2" {
				t.Errorf("project = %v", got["project"])
			}
			if got["content"] != "raw text" {
				t.Errorf("content = %v", got["content"])
			}
			if got["type"] != "capture" {
				t.Errorf("type = %v", got["type"])
			}
			// Title was empty in input; HTTPClient must derive a
			// non-empty title at the wire layer (Engram requires it).
			if got["title"] == "" || got["title"] == nil {
				t.Errorf("title was not derived; engram requires non-empty title")
			}
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"id":334,"status":"saved"}`))

		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	h := NewHTTPClient(srv.URL, "", 2*time.Second)
	id, err := h.Save(context.Background(), Observation{
		Title:   "",
		Content: "raw text",
		Project: "silo2",
		Type:    "capture",
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !sessionCalled || !observationCalled {
		t.Errorf("missing call: session=%v observation=%v", sessionCalled, observationCalled)
	}
	if id != "334" {
		t.Errorf("returned id = %q, want \"334\"", id)
	}
}

func TestHTTPClient_Save_ForwardsWhyForFutureCompat(t *testing.T) {
	// Engram v1.15.13 silently drops `why` (the column doesn't exist),
	// but Silo forwards it anyway so a future Engram version with native
	// `why` support starts persisting it without a client change. The
	// payload presence is the contract; what Engram does with it is not.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sessions" {
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"id":"x","status":"created"}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"why":"because I want to remember"`) {
			t.Errorf("payload missing why field: %s", body)
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":1,"status":"saved"}`))
	}))
	defer srv.Close()

	h := NewHTTPClient(srv.URL, "", 2*time.Second)
	if _, err := h.Save(context.Background(), Observation{
		Title:   "t", Content: "c", Project: "p",
		Why: "because I want to remember",
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func TestHTTPClient_Save_NeverMergesWhyIntoContent(t *testing.T) {
	// Memory is sacred: Why is metadata, not content. Pin that the wire
	// payload's "content" field is untouched even with a non-empty Why.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sessions" {
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		var got map[string]any
		_ = json.Unmarshal(body, &got)
		if got["content"] != "raw and clean" {
			t.Errorf("content was contaminated: %v", got["content"])
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":1,"status":"saved"}`))
	}))
	defer srv.Close()

	h := NewHTTPClient(srv.URL, "", 2*time.Second)
	if _, err := h.Save(context.Background(), Observation{
		Title: "t", Content: "raw and clean", Project: "p",
		Why: "this should NOT appear inside content",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPClient_Save_DerivesTitleWhenEmpty(t *testing.T) {
	// `silo save` constructs Observation with Title="". Engram requires
	// a non-empty title. HTTPClient.Save MUST derive one at the wire
	// layer (without mutating the caller's local Observation, and without
	// requiring callers to invent a title).
	var seenTitle string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sessions" {
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		var got map[string]any
		_ = json.Unmarshal(body, &got)
		seenTitle, _ = got["title"].(string)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":1,"status":"saved"}`))
	}))
	defer srv.Close()

	h := NewHTTPClient(srv.URL, "", 2*time.Second)
	if _, err := h.Save(context.Background(), Observation{
		Content: "MVVM-C navigation insight worth remembering",
		Project: "silo2",
	}); err != nil {
		t.Fatal(err)
	}
	if seenTitle == "" {
		t.Error("derived title is empty")
	}
	if !strings.Contains(seenTitle, "MVVM-C") {
		t.Errorf("derived title should reflect content, got %q", seenTitle)
	}
}

func TestHTTPClient_Save_EmptyContentRejectedLocally(t *testing.T) {
	// Defensive: empty content would round-trip into a 400 from Engram.
	// Catch it locally with a clearer error so callers don't blame the wire.
	h := NewHTTPClient("http://127.0.0.1:0", "", 0)
	_, err := h.Save(context.Background(), Observation{Project: "p"})
	if err == nil {
		t.Fatal("expected error on empty content")
	}
	if strings.Contains(err.Error(), "127.0.0.1") {
		t.Errorf("error reached the network instead of failing locally: %v", err)
	}
}

func TestHTTPClient_Save_SurfacesEngramErrors(t *testing.T) {
	// 4xx/5xx with body must be surfaced verbatim so debugging is sane.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/sessions" {
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":"constraint failed: FOREIGN KEY constraint failed (787)"}`))
	}))
	defer srv.Close()

	h := NewHTTPClient(srv.URL, "", 2*time.Second)
	_, err := h.Save(context.Background(), Observation{
		Title: "t", Content: "c", Project: "p",
	})
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Errorf("engram error body should surface: %v", err)
	}
	if errors.Is(err, ErrSaveUnsupported) {
		t.Errorf("real errors must not look like ErrSaveUnsupported")
	}
}

func TestHTTPClient_Save_SessionFailureAborts(t *testing.T) {
	// If session bootstrap fails (auth, network), we must not proceed
	// to POST /observations — that would 500 with FK violation and
	// confuse the user about the real cause.
	var observationCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sessions":
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		case "/observations":
			observationCalled = true
			w.WriteHeader(201)
		}
	}))
	defer srv.Close()

	h := NewHTTPClient(srv.URL, "wrong-key", 2*time.Second)
	_, err := h.Save(context.Background(), Observation{Title: "t", Content: "c", Project: "p"})
	if err == nil {
		t.Fatal("expected error")
	}
	if observationCalled {
		t.Error("observation POST happened despite session failure; that would mask the real cause")
	}
}
