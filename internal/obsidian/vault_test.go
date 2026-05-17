package obsidian

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteNoteAt_HappyPath(t *testing.T) {
	dir := t.TempDir()
	v := NewVault(dir)
	if err := v.EnsureDir(); err != nil {
		t.Fatal(err)
	}
	if err := v.WriteNoteAt("Raw/Observations", "x.md", "hi"); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "Raw", "Observations", "x.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hi" {
		t.Errorf("got %q", got)
	}
}

func TestWriteNoteAt_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	v := NewVault(dir)
	_ = v.EnsureDir()

	cases := []struct{ subdir, name string }{
		{"..", "x.md"},
		{"Raw/../..", "x.md"},
		{"Raw", "../x.md"},
		{"", "../escape.md"},
		{"/abs/path", "x.md"},
		{"Raw", "/abs.md"},
	}
	for _, c := range cases {
		err := v.WriteNoteAt(c.subdir, c.name, "x")
		if err == nil {
			t.Errorf("expected error for subdir=%q name=%q", c.subdir, c.name)
		}
	}
}
