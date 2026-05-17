package engram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPClient talks to a real Engram instance over its built-in HTTP API,
// which is started with: `engram serve [port]` (default port 7437).
//
// REAL wire contract (verified against engram v1.15.13):
//
//	GET /health
//	  → {"service":"engram","status":"ok","version":"..."}
//
//	GET /search?q=<required>[&project=<p>][&type=<t>][&scope=<s>][&limit=<n>]
//	  → []wireObservation     // FTS5 ranked. q is REQUIRED; empty/missing q → 400.
//	  → null                  // when there are zero matches (NOT [])
//
//	GET /export[?project=<p>]
//	  → {                                  // FULL project dump
//	      "version": "...",
//	      "exported_at": "...",
//	      "sessions":     [...],
//	      "observations": [wireObservation, ...],
//	      "prompts":      [...]
//	    }
//
// IMPORTANT — corrections vs the previous adapter assumption:
//
//   - There is NO `GET /observations` endpoint. `/observations` exists but
//     accepts POST only (it is the CREATE endpoint, not a query). GETting it
//     returns 405. Earlier drafts of this client targeted it and silently
//     broke against real Engram.
//   - `/search` REQUIRES a non-empty `q`. There is no documented "list all
//     observations for project X" via /search. For that exhaustive case we
//     use `/export?project=<p>` and read the `observations[]` slice. The
//     export endpoint is read-only, fast against a local store, and is the
//     only HTTP path that returns every observation of a project without
//     FTS5 ranking constraints.
//   - Observation IDs come as JSON integers, not strings.
//   - Timestamps are SQL DATETIME ("2006-01-02 15:04:05"), not RFC3339.
//   - 4xx/5xx bodies are `{"error":"..."}` JSON; we surface the snippet.
//
// Everything Engram-specific stays here so the rest of the system keeps
// consuming the stable engram.Observation shape (string ID, time.Time).
type HTTPClient struct {
	endpoint string
	apiKey   string
	http     *http.Client
}

func NewHTTPClient(endpoint, apiKey string, timeout time.Duration) *HTTPClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPClient{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   strings.TrimSpace(apiKey),
		http:     &http.Client{Timeout: timeout},
	}
}

// --- Client interface implementation ---------------------------------------

// Search hits GET /search?q=<q>. Engram requires a non-empty q; passing an
// empty query is a caller bug and surfaces as the Engram 400 verbatim
// (rather than silently returning an unrelated dataset).
func (h *HTTPClient) Search(ctx context.Context, query string) ([]Observation, error) {
	q := url.Values{}
	q.Set("q", strings.TrimSpace(query))
	raw, err := h.getRaw(ctx, "/search", q)
	if err != nil {
		return nil, err
	}
	return decodeObservationArray(raw)
}

// Context returns every observation belonging to `project`. Implemented on
// top of GET /export?project=<project> because Engram intentionally does
// not expose a "list all observations" query endpoint — /search is FTS5
// and /observations is write-only. /export is the canonical full-project
// projection and is what `engram export` itself uses.
func (h *HTTPClient) Context(ctx context.Context, project string) ([]Observation, error) {
	q := url.Values{}
	if p := strings.TrimSpace(project); p != "" {
		q.Set("project", p)
	}
	raw, err := h.getRaw(ctx, "/export", q)
	if err != nil {
		return nil, err
	}

	// /export wraps observations inside an envelope. We decode only the
	// observations slice; sessions and prompts are intentionally ignored
	// here so Silo stays a projection of *observations*, not a mirror of
	// Engram's full graph.
	var env struct {
		Observations []wireObservation `json:"observations"`
	}
	body := bytes.TrimSpace(raw)
	if len(body) == 0 || bytes.Equal(body, []byte("null")) {
		return []Observation{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&env); err != nil {
		return nil, fmt.Errorf("engram: decode /export envelope: %w", err)
	}

	out := make([]Observation, 0, len(env.Observations))
	for _, w := range env.Observations {
		out = append(out, w.toObservation())
	}
	return out, nil
}

// --- Wire format (isolated to this file) -----------------------------------

// wireObservation mirrors Engram's HTTP JSON shape. Kept private on purpose:
// no other package should ever see Engram's snake_case integers or SQL
// timestamps. Fields not consumed downstream (rank, revision_count,
// duplicate_count, last_seen_at, topic_key, scope, session_id) are dropped
// silently — adding them here would just bloat the type without changing
// behavior, since identity/markdown only read the stable Observation shape.
type wireObservation struct {
	ID        json.Number `json:"id"`
	Title     string      `json:"title"`
	Type      string      `json:"type"`
	Content   string      `json:"content"`
	Project   string      `json:"project"`
	TopicKey  string      `json:"topic_key"`
	CreatedAt string      `json:"created_at"`
	SyncID    string      `json:"sync_id"`
}

// engramTimeLayout matches the SQL DATETIME format Engram returns over HTTP.
const engramTimeLayout = "2006-01-02 15:04:05"

func (w wireObservation) toObservation() Observation {
	id := w.ID.String()
	if id == "" {
		// Fall back to sync_id when the numeric id is unavailable. Should not
		// happen with current Engram, but defensive parsing avoids dropping
		// rows for future schema drift.
		id = w.SyncID
	}

	var created time.Time
	if s := strings.TrimSpace(w.CreatedAt); s != "" {
		if t, err := time.Parse(engramTimeLayout, s); err == nil {
			created = t.UTC()
		} else if t, err := time.Parse(time.RFC3339, s); err == nil {
			// Tolerate RFC3339 in case Engram changes its format.
			created = t.UTC()
		}
		// On parse failure we silently leave created as zero. The downstream
		// renderer already handles zero CreatedAt.
	}

	return Observation{
		ID:        id,
		Title:     w.Title,
		Type:      w.Type,
		Content:   w.Content,
		Project:   w.Project,
		TopicKey:  strings.TrimSpace(w.TopicKey),
		CreatedAt: created,
	}
}

// --- Internal --------------------------------------------------------------

// getRaw performs the HTTP GET and returns the raw body with error context
// already attached. It does NOT decode — callers pick the right shape
// (array for /search, envelope for /export) so we don't smear two formats
// into one fragile decoder.
func (h *HTTPClient) getRaw(ctx context.Context, path string, params url.Values) ([]byte, error) {
	full := h.endpoint + path
	if encoded := params.Encode(); encoded != "" {
		full += "?" + encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, fmt.Errorf("engram: build request %s: %w", full, err)
	}
	req.Header.Set("Accept", "application/json")
	if h.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}

	resp, err := h.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("engram: GET %s: %w", full, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20)) // 64MB cap (full export can be large)
	if err != nil {
		return nil, fmt.Errorf("engram: read response from %s: %w", full, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(raw))
		if len(snippet) > 512 {
			snippet = snippet[:512] + "…"
		}
		return nil, fmt.Errorf("engram: GET %s returned %d: %s", full, resp.StatusCode, snippet)
	}
	return raw, nil
}

// decodeObservationArray handles the /search response shape: either a
// JSON array of wireObservation, or the literal `null` Engram returns
// when there are zero matches.
func decodeObservationArray(raw []byte) ([]Observation, error) {
	body := bytes.TrimSpace(raw)
	if len(body) == 0 || bytes.Equal(body, []byte("null")) {
		return []Observation{}, nil
	}

	var wires []wireObservation
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&wires); err != nil {
		return nil, fmt.Errorf("engram: decode observation array: %w", err)
	}

	out := make([]Observation, 0, len(wires))
	for _, w := range wires {
		out = append(out, w.toObservation())
	}
	return out, nil
}
