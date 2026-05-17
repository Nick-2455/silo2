package curated

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Pure parsing tests (no filesystem) ------------------------------------

func TestParseNote_PristineSeed_NotUseful(t *testing.T) {
	// This is the exact shape `silo curate` writes for a fresh seed.
	seed := `---
type: curated
generated_by: silo
source: engram
topic_key: architecture/silo-design
---

# Silo Design

## Summary

TODO: Write human-curated summary.

## Related Observations

- [[Raw/Observations/silo-design]]

## Notes

TODO.
`
	n := parseNote(seed)
	if n.useful {
		t.Errorf("pristine seed must not be useful, but parseNote said useful")
	}
	if n.title != "Silo Design" {
		t.Errorf("expected title 'Silo Design', got %q", n.title)
	}
}

func TestParseNote_HumanProse_IsUseful(t *testing.T) {
	note := `---
type: curated
---

# Engram HTTP API

## Summary

Silo talks to Engram via /export?project=. Key gotcha: GET /observations
returns 405 because /observations is POST-only.

## Related Observations

- [[Raw/Observations/x]]
- [[Raw/Observations/y]]

## Notes

TODO.
`
	n := parseNote(note)
	if !n.useful {
		t.Fatal("human prose in Summary must count as useful")
	}
	if !strings.Contains(n.content, "Silo talks to Engram") {
		t.Errorf("content should preserve human prose, got:\n%s", n.content)
	}
	if strings.Contains(n.content, "Raw/Observations/x") {
		t.Errorf("Related Observations links must be stripped from content, got:\n%s", n.content)
	}
}

func TestParseNote_OnlyRelatedLinks_NotUseful(t *testing.T) {
	// Edge case: Summary is empty TODO, Notes is empty TODO, only the
	// auto-generated link list has text. Must NOT count as human signal.
	note := `---
type: curated
---

# X

## Summary

TODO.

## Related Observations

- [[Raw/Observations/a]]
- [[Raw/Observations/b]]
- [[Raw/Observations/c]]

## Notes

TODO.
`
	if parseNote(note).useful {
		t.Error("Related Observations alone must not make a curated note useful")
	}
}

func TestParseNote_BulletListIsUseful(t *testing.T) {
	note := `---
type: curated
---

# Career highlights

## Notes

- shipped Engram MCP integration
- led migration to YOLO detector
`
	if !parseNote(note).useful {
		t.Error("bullet-list prose must count as useful")
	}
}

func TestParseNote_TodoVariants_NotUseful(t *testing.T) {
	cases := []string{
		"# X\n\nTODO",
		"# X\n\nTODO.",
		"# X\n\nTODO:",
		"# X\n\nTODO: write me",
		"# X\n\ntodo - flesh out",
		"# X\n\n- TODO: bullet placeholder",
	}
	for _, c := range cases {
		if parseNote(c).useful {
			t.Errorf("TODO variant should not be useful:\n%s", c)
		}
	}
}

func TestStripFrontmatter(t *testing.T) {
	in := "---\ntype: x\n---\nbody"
	got := stripFrontmatter(in)
	if got != "body" {
		t.Errorf("got %q", got)
	}
	// No frontmatter → unchanged.
	if stripFrontmatter("# H\nbody") != "# H\nbody" {
		t.Error("non-frontmatter input must be unchanged")
	}
}

// --- LoadCurated tests (filesystem) ----------------------------------------

func TestLoadCurated_MissingDir_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	obs, err := LoadCurated(dir, "silo2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs) != 0 {
		t.Errorf("expected empty slice when Curated/ does not exist, got %d", len(obs))
	}
}

func TestLoadCurated_OnlyPristineSeeds_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	seedPath := filepath.Join(dir, "Curated/Architecture/silo-design.md")
	_ = os.MkdirAll(filepath.Dir(seedPath), 0o755)
	_ = os.WriteFile(seedPath, []byte(`---
type: curated
---

# Silo Design

## Summary

TODO: Write human-curated summary.

## Notes

TODO.
`), 0o644)

	obs, err := LoadCurated(dir, "silo2")
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
	}
	if len(obs) != 0 {
		t.Errorf("pristine seeds must not surface as observations, got %d", len(obs))
	}
}

func TestLoadCurated_SkipsReadme(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "Curated/Career"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "Curated/Career/README.md"), []byte(`---
type: curated-index
---

# Career

This folder holds notes. Plenty of human-readable text here.
`), 0o644)

	obs, err := LoadCurated(dir, "silo2")
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
	}
	if len(obs) != 0 {
		t.Errorf("README files must be skipped, got %d obs", len(obs))
	}
}

func TestLoadCurated_HumanContent_ProducesSyntheticObservation(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "Curated/Identity/profile.md")
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, []byte(`---
type: curated
---

# Profile

## Summary

Nicolas is a Go architect focused on Engram and Obsidian tooling.

## Related Observations

- [[Raw/Observations/x]]
`), 0o644)

	obs, err := LoadCurated(dir, "silo2")
	if err != nil {
		t.Fatalf("LoadCurated: %v", err)
	}
	if len(obs) != 1 {
		t.Fatalf("expected 1 synthetic obs, got %d", len(obs))
	}
	o := obs[0]
	if o.ID != "curated:Curated/Identity/profile.md" {
		t.Errorf("unexpected ID: %q", o.ID)
	}
	if o.Type != "curated" {
		t.Errorf("expected type=curated, got %q", o.Type)
	}
	if o.Project != "silo2" {
		t.Errorf("expected project=silo2, got %q", o.Project)
	}
	if o.Title != "Profile" {
		t.Errorf("expected title=Profile, got %q", o.Title)
	}
	if !strings.Contains(o.Content, "Nicolas is a Go architect") {
		t.Errorf("content missing human prose:\n%s", o.Content)
	}
}

func TestLoadCurated_DeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.md", "a.md", "c.md"} {
		p := filepath.Join(dir, "Curated/Architecture", name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("---\ntype: curated\n---\n\n# "+name+"\n\n## Summary\n\nreal text here\n"), 0o644)
	}
	first, _ := LoadCurated(dir, "silo2")
	second, _ := LoadCurated(dir, "silo2")
	if len(first) != 3 || len(second) != 3 {
		t.Fatalf("expected 3, got %d / %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Errorf("non-deterministic order at %d: %q vs %q", i, first[i].ID, second[i].ID)
		}
	}
}
