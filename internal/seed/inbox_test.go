package seed

import (
	"os"
	"path/filepath"
	"testing"
)

// The inbox is the W2 surface. Its job is dumb and reliable: scan the
// two on-disk folders, parse the `status:` field from frontmatter, and
// return a summary. No promotion logic lives here — that is a human act.

func TestScanInbox_EmptyVaultReturnsZeroes(t *testing.T) {
	dir := t.TempDir()
	res, err := ScanInbox(dir)
	if err != nil {
		t.Fatalf("ScanInbox: %v", err)
	}
	if res.Total() != 0 {
		t.Errorf("expected empty inbox, got total=%d", res.Total())
	}
}

func TestScanInbox_CountsByStatusFromFrontmatter(t *testing.T) {
	dir := t.TempDir()

	// 2 open seeds in open/, 1 deferred in open/ (frontmatter wins),
	// 1 archived seed in archive/ with status: discarded.
	mustWriteSeed(t, dir, "Inbox/open/seed-a.md", "open")
	mustWriteSeed(t, dir, "Inbox/open/seed-b.md", "open")
	mustWriteSeed(t, dir, "Inbox/open/seed-c.md", "deferred")
	mustWriteSeed(t, dir, "Inbox/archive/seed-d.md", "discarded")

	res, err := ScanInbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Open != 2 {
		t.Errorf("Open = %d, want 2", res.Open)
	}
	if res.Deferred != 1 {
		t.Errorf("Deferred = %d, want 1", res.Deferred)
	}
	if res.Discarded != 1 {
		t.Errorf("Discarded = %d, want 1", res.Discarded)
	}
	if res.Approved != 0 {
		t.Errorf("Approved = %d, want 0", res.Approved)
	}
	if res.Total() != 4 {
		t.Errorf("Total = %d, want 4", res.Total())
	}
}

func TestScanInbox_OpenListIsSortedByFilename(t *testing.T) {
	// CLI output must be stable across runs. Sort alphabetically.
	dir := t.TempDir()
	mustWriteSeed(t, dir, "Inbox/open/seed-zzz.md", "open")
	mustWriteSeed(t, dir, "Inbox/open/seed-aaa.md", "open")
	mustWriteSeed(t, dir, "Inbox/open/seed-mmm.md", "open")

	res, err := ScanInbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.OpenFiles) != 3 {
		t.Fatalf("OpenFiles len = %d, want 3", len(res.OpenFiles))
	}
	if res.OpenFiles[0] != "seed-aaa.md" || res.OpenFiles[2] != "seed-zzz.md" {
		t.Errorf("OpenFiles not sorted: %v", res.OpenFiles)
	}
}

func TestScanInbox_MissingStatusIsTreatedAsOpen(t *testing.T) {
	// Defensive: a human may hand-create a seed without frontmatter.
	// We default to "open" so the file shows up in triage instead of
	// silently disappearing.
	dir := t.TempDir()
	mustWriteRaw(t, dir, "Inbox/open/seed-x.md", "no frontmatter here\n")

	res, err := ScanInbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Open != 1 {
		t.Errorf("Open = %d, want 1 (status-less file should default to open)", res.Open)
	}
}

func TestScanInbox_NonSeedFilesIgnored(t *testing.T) {
	// READMEs and any non-.md file should not be counted.
	dir := t.TempDir()
	mustWriteRaw(t, dir, "Inbox/open/README.md", "# Inbox\n")
	mustWriteRaw(t, dir, "Inbox/open/notes.txt", "ignore me")
	mustWriteSeed(t, dir, "Inbox/open/seed-1.md", "open")

	res, err := ScanInbox(dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Open != 1 {
		t.Errorf("Open = %d, want 1 (README and .txt must be ignored)", res.Open)
	}
}

// --- test helpers ---

func mustWriteSeed(t *testing.T, vault, rel, status string) {
	t.Helper()
	body := "---\ntype: seed\nstatus: " + status + "\ngenerated_by: silo\nsource_observation: obs-x\n---\n\n# T\n"
	mustWriteRaw(t, vault, rel, body)
}

func mustWriteRaw(t *testing.T, vault, rel, body string) {
	t.Helper()
	full := filepath.Join(vault, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
