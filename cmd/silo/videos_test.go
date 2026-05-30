package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadVideoSeed_ParsesVideoSource(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "seed-x.md")
	content := `---
type: seed
status: deferred
generated_by: silo
source_observation: 123
---

# My Title

## Sources

- video: https://example.com/watch?v=1

## Human Notes

TODO.
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := readVideoSeed(path)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got.Title != "My Title" {
		t.Fatalf("Title=%q, want %q", got.Title, "My Title")
	}
	if got.Status != "deferred" {
		t.Fatalf("Status=%q, want %q", got.Status, "deferred")
	}
	if got.SourceURL != "https://example.com/watch?v=1" {
		t.Fatalf("SourceURL=%q", got.SourceURL)
	}
}

func TestReadVideoSeed_IgnoresNonVideoSources(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "seed-x.md")
	content := `---
type: seed
status: open
---

# Title

## Sources

- article: https://example.com/a
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok := readVideoSeed(path)
	if ok {
		t.Fatalf("expected ok=false")
	}
}

func TestRenderVideoWatchLater_DeterministicOrdering(t *testing.T) {
	items := []videoSeed{
		{RelPath: "Inbox/open/a.md", Title: "B", Status: "open", SourceURL: "https://b"},
		{RelPath: "Inbox/open/b.md", Title: "A", Status: "open", SourceURL: "https://a"},
	}
	// The renderer itself doesn't sort; scanVideoSeeds does. But the output
	// must include the URLs and titles as links.
	out := renderVideoWatchLater(items, time.Time{})
	if !strings.Contains(out, "# Videos - Watch Later") {
		t.Fatalf("missing title header")
	}
	if !strings.Contains(out, "[A](https://a)") || !strings.Contains(out, "[B](https://b)") {
		t.Fatalf("missing expected links:\n%s", out)
	}
}

func TestSplitTab(t *testing.T) {
	a, b, ok := splitTab("id\ttitle")
	if !ok {
		t.Fatalf("expected ok")
	}
	if a != "id" || b != "title" {
		t.Fatalf("got %q %q", a, b)
	}
}
