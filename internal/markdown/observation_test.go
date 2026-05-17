package markdown

import (
	"strings"
	"testing"
	"time"

	"github.com/nicolasperalta/silo2/internal/engram"
)

func TestSlug(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Hello World", "hello-world"},
		{"  Spaces   trimmed  ", "spaces-trimmed"},
		{"Engram & Silo: notes / drafts!", "engram-silo-notes-drafts"},
		{"___weird---chars+++", "weird-chars"},
		{"Café résumé piñata", "caf-r-sum-pi-ata"},
		{"", ""},
		{"////", ""},
		{"a/b/../c", "a-b-c"},
		{"....", ""},
		{strings.Repeat("a", 200), strings.Repeat("a", 80)},
	}
	for _, c := range cases {
		got := Slug(c.in)
		if got != c.want {
			t.Errorf("Slug(%q) = %q, want %q", c.in, got, c.want)
		}
		// Defensive invariants: never path-like.
		if strings.ContainsAny(got, "/\\") {
			t.Errorf("Slug(%q) leaked path separator: %q", c.in, got)
		}
		if got == ".." || strings.Contains(got, "..") {
			t.Errorf("Slug(%q) produced traversal: %q", c.in, got)
		}
	}
}

func TestRenderObservations_EmptyTitle(t *testing.T) {
	obs := []engram.Observation{
		{ID: "obs-x1", Title: "", Type: "note", Content: "body", Project: "silo2"},
	}
	out, err := RenderObservations(obs)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out["observation-obs-x1.md"]; !ok {
		t.Fatalf("expected fallback filename, got keys: %v", keys(out))
	}
}

func TestRenderObservations_Collision(t *testing.T) {
	obs := []engram.Observation{
		{ID: "obs-a", Title: "Silo Design", Type: "note", Project: "silo2"},
		{ID: "obs-b", Title: "silo--design", Type: "note", Project: "silo2"},
		{ID: "obs-c", Title: "  SILO   DESIGN ", Type: "note", Project: "silo2"},
	}
	out, err := RenderObservations(obs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3 distinct files, got %d: %v", len(out), keys(out))
	}
	// First-by-ID (obs-a) keeps bare slug; others get ID suffix.
	if _, ok := out["silo-design.md"]; !ok {
		t.Errorf("expected silo-design.md to win, got: %v", keys(out))
	}
	if _, ok := out["silo-design-obs-b.md"]; !ok {
		t.Errorf("expected silo-design-obs-b.md, got: %v", keys(out))
	}
	if _, ok := out["silo-design-obs-c.md"]; !ok {
		t.Errorf("expected silo-design-obs-c.md, got: %v", keys(out))
	}
}

func TestRenderObservations_Idempotent(t *testing.T) {
	obs := []engram.Observation{
		{ID: "obs-2", Title: "Beta", Content: "b", Project: "silo2", CreatedAt: time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)},
		{ID: "obs-1", Title: "Alpha", Content: "a", Project: "silo2", CreatedAt: time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)},
	}
	out1, err := RenderObservations(obs)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := RenderObservations(obs)
	if err != nil {
		t.Fatal(err)
	}
	if len(out1) != len(out2) {
		t.Fatalf("len mismatch: %d vs %d", len(out1), len(out2))
	}
	for k, v := range out1 {
		if out2[k] != v {
			t.Errorf("content drift for %q", k)
		}
	}
}

func TestRenderObservations_Empty_WritesReadme(t *testing.T) {
	out, err := RenderObservations(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out["README.md"] == "" {
		t.Fatalf("expected single README.md, got: %v", keys(out))
	}
	if !strings.Contains(out["README.md"], "No observations") {
		t.Errorf("README missing explainer text")
	}
}

func TestObservationFilename_NoPathSeparators(t *testing.T) {
	obs := engram.Observation{ID: "../escape", Title: "../../etc/passwd"}
	name := observationFilename(obs, map[string]bool{})
	if strings.ContainsAny(name, "/\\") {
		t.Errorf("filename leaked separator: %q", name)
	}
	if strings.Contains(name, "..") {
		t.Errorf("filename contains traversal: %q", name)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
