package engram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HTTPClient.Save wires `silo save` to the real Engram HTTP API.
//
// Wire contract (verified against engram v1.15.13 — `engram serve`):
//
//   POST /sessions
//     Body : {"id": "...", "project": "...", "directory": "..."?}
//     201  : {"id":"...","status":"created"}
//     Upsert-safe: re-POSTing the same id returns 201 again (no 409).
//     400  : {"error":"id and project are required"}
//
//   POST /observations
//     Body : {
//       "session_id": <required>,
//       "title":      <required, non-empty>,
//       "content":    <required, non-empty>,
//       "project":    <optional>,
//       "type":       <optional>,
//       "topic_key":  <optional>,
//       "scope":      <optional>,
//       "why":        <forwarded; silently dropped by current Engram>
//     }
//     201  : {"id": <int>, "status": "saved"}
//     400  : {"error":"session_id, title, and content are required"}
//     500  : {"error":"constraint failed: FOREIGN KEY constraint failed (787)"}
//            (raised when session_id does not exist; Save bootstraps a
//             session first to avoid this.)
//
// Memory layer contract enforced here:
//
//   - `content` is sent verbatim. Why is NEVER merged into content.
//   - `why` is forwarded as its own field. Current Engram has no `why`
//     column and silently discards it; the value survives in the seed
//     file's "Capture Why" section. When a future Engram persists `why`,
//     Silo starts roundtripping it without a client change.
//   - `title` is required by Engram but empty in fresh `silo save`
//     captures. We derive a title at the wire layer from content so the
//     caller's local Observation is not mutated and the seed renderer
//     keeps applying its own (independent) title rules.

// silo-save sessions group every capture for a project under a single
// long-lived session. Same shape used by other tools that integrate
// with Engram (e.g. `manual-save-{project}`); the prefix differs so an
// Engram operator can spot which client made the writes.
const sessionIDPrefix = "silo-save-"

// titleFallbackMax bounds the derived title; 60 chars keeps it readable
// in `engram` CLI listings without truncating mid-word too aggressively.
const titleFallbackMax = 60

// Save creates the session (idempotently) and the observation in one
// logical operation. It returns the integer Engram ID as a string to
// match the rest of the Observation.ID convention used in this package.
func (h *HTTPClient) Save(ctx context.Context, obs Observation) (string, error) {
	if strings.TrimSpace(obs.Content) == "" {
		return "", errors.New("engram: Save requires non-empty Content")
	}
	project := strings.TrimSpace(obs.Project)
	if project == "" {
		return "", errors.New("engram: Save requires non-empty Project")
	}

	sessionID := sessionIDPrefix + project

	// 1. Bootstrap the session. Upsert-safe on Engram v1.15.13, so we
	// can call it on every save without tracking state client-side.
	if err := h.upsertSession(ctx, sessionID, project); err != nil {
		return "", fmt.Errorf("engram: bootstrap session: %w", err)
	}

	// 2. POST the observation.
	id, err := h.postObservation(ctx, obs, sessionID)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (h *HTTPClient) upsertSession(ctx context.Context, sessionID, project string) error {
	body, err := json.Marshal(map[string]string{
		"id":      sessionID,
		"project": project,
	})
	if err != nil {
		return err
	}
	_, err = h.postJSON(ctx, "/sessions", body)
	return err
}

// observationPayload is the wire shape sent to POST /observations. The
// struct uses omitempty for optional fields so we don't send empty
// strings that Engram might reject in a future tightened schema.
//
// `why` is intentionally always emitted when non-empty even though
// current Engram drops it: forward-compatibility with no client change.
type observationPayload struct {
	SessionID string `json:"session_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Project   string `json:"project,omitempty"`
	Type      string `json:"type,omitempty"`
	TopicKey  string `json:"topic_key,omitempty"`
	Why       string `json:"why,omitempty"`
}

// observationResponse mirrors {"id": <int>, "status": "saved"}.
type observationResponse struct {
	ID     json.Number `json:"id"`
	Status string      `json:"status"`
}

func (h *HTTPClient) postObservation(ctx context.Context, obs Observation, sessionID string) (string, error) {
	payload := observationPayload{
		SessionID: sessionID,
		Title:     wireTitle(obs),
		Content:   obs.Content, // verbatim — Memory is sacred
		Project:   strings.TrimSpace(obs.Project),
		Type:      strings.TrimSpace(obs.Type),
		TopicKey:  strings.TrimSpace(obs.TopicKey),
		Why:       strings.TrimSpace(obs.Why),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	raw, err := h.postJSON(ctx, "/observations", body)
	if err != nil {
		return "", err
	}

	var resp observationResponse
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&resp); err != nil {
		return "", fmt.Errorf("engram: decode /observations response: %w", err)
	}
	id := resp.ID.String()
	if id == "" {
		return "", fmt.Errorf("engram: /observations response missing id: %s", string(raw))
	}
	return id, nil
}

// wireTitle returns a non-empty title for the POST payload, deriving one
// from content when the caller did not supply a title. The local
// Observation is never mutated — this is a wire-layer concern.
func wireTitle(obs Observation) string {
	if t := strings.TrimSpace(obs.Title); t != "" {
		return t
	}
	content := strings.TrimSpace(obs.Content)
	if content == "" {
		return "Untitled capture"
	}
	// Take the first line, then cap to titleFallbackMax runes.
	if i := strings.IndexByte(content, '\n'); i > 0 {
		content = content[:i]
	}
	runes := []rune(content)
	if len(runes) > titleFallbackMax {
		runes = runes[:titleFallbackMax]
		return strings.TrimRight(string(runes), " \t") + "…"
	}
	return string(runes)
}

// postJSON is the shared write helper. Symmetric with getRaw in
// http_client.go: returns the raw body and a context-rich error on
// non-2xx, with the response body snippet surfaced into the message.
func (h *HTTPClient) postJSON(ctx context.Context, path string, body []byte) ([]byte, error) {
	full := h.endpoint + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, full, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("engram: build POST %s: %w", full, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if h.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}

	resp, err := h.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("engram: POST %s: %w", full, err)
	}
	defer resp.Body.Close()

	// Same 64MB cap as GET; write paths in practice are tiny but the
	// shared limit avoids surprise.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("engram: read response from %s: %w", full, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(raw))
		if len(snippet) > 512 {
			snippet = snippet[:512] + "…"
		}
		return nil, fmt.Errorf("engram: POST %s returned %d: %s", full, resp.StatusCode, snippet)
	}
	return raw, nil
}

