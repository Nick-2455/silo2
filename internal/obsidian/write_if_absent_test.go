package obsidian

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteNoteIfAbsent_WritesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	v := NewVault(dir)

	res, err := v.WriteNoteIfAbsent("Curated/Architecture", "silo-design.md", "seed content")
	if err != nil {
		t.Fatalf("WriteNoteIfAbsent: %v", err)
	}
	if res != Written {
		t.Errorf("expected Written, got %v", res)
	}

	b, err := os.ReadFile(filepath.Join(dir, "Curated/Architecture/silo-design.md"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(b) != "seed content" {
		t.Errorf("unexpected content: %q", b)
	}
}

func TestWriteNoteIfAbsent_SkipsWhenPresent_PreservesHumanEdits(t *testing.T) {
	dir := t.TempDir()
	v := NewVault(dir)

	// Human writes a curated note by hand.
	humanContent := "## My carefully crafted notes\n\nDo not touch."
	if _, err := v.WriteNoteIfAbsent("Curated/Architecture", "silo-design.md", humanContent); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Silo runs curate again with a different seed → must be skipped.
	res, err := v.WriteNoteIfAbsent("Curated/Architecture", "silo-design.md", "DIFFERENT SEED")
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if res != Skipped {
		t.Errorf("expected Skipped, got %v", res)
	}

	b, _ := os.ReadFile(filepath.Join(dir, "Curated/Architecture/silo-design.md"))
	if string(b) != humanContent {
		t.Errorf("human edits were overwritten! got: %q", b)
	}
}

func TestWriteNoteIfAbsent_RefusesEscape(t *testing.T) {
	dir := t.TempDir()
	v := NewVault(dir)

	if _, err := v.WriteNoteIfAbsent("..", "evil.md", "x"); err == nil {
		t.Error("expected error on parent traversal subdir")
	}
	if _, err := v.WriteNoteIfAbsent("ok", "../escape.md", "x"); err == nil {
		t.Error("expected error on parent traversal name")
	}
}
