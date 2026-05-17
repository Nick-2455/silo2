package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nicolasperalta/silo2/internal/engram"
	"github.com/nicolasperalta/silo2/internal/obsidian"
	"github.com/nicolasperalta/silo2/internal/seed"
)

func TestImportWiki_IgnoresReadmeByDefault(t *testing.T) {
	legacy := t.TempDir()
	_ = os.MkdirAll(filepath.Join(legacy, "sub"), 0o755)
	if err := os.WriteFile(filepath.Join(legacy, "README.md"), []byte("# Root readme"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "sub", "note.md"), []byte("# Title\n\nBody"), 0o644); err != nil {
		t.Fatal(err)
	}

	vaultDir := t.TempDir()
	deps := importWikiDeps{
		Client:    engram.NewMockClient(),
		Generator: seed.NewMockGenerator(),
		Vault:     obsidian.NewVault(vaultDir),
		Stdout:    &bytes.Buffer{},
	}

	res, err := importWikiCore(context.Background(), deps, importWikiInput{
		Project: "silo2",
		Root:    legacy,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesFound != 1 {
		t.Fatalf("expected 1 file found, got %d", res.FilesFound)
	}

	matches, _ := filepath.Glob(filepath.Join(vaultDir, "Inbox/open/seed-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 seed, got %d (%v)", len(matches), matches)
	}
}

func TestImportWiki_IncludeReadmeWorks(t *testing.T) {
	legacy := t.TempDir()
	if err := os.WriteFile(filepath.Join(legacy, "README.md"), []byte("# Root readme"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "note.md"), []byte("# Note"), 0o644); err != nil {
		t.Fatal(err)
	}

	vaultDir := t.TempDir()
	deps := importWikiDeps{
		Client:    engram.NewMockClient(),
		Generator: seed.NewMockGenerator(),
		Vault:     obsidian.NewVault(vaultDir),
		Stdout:    &bytes.Buffer{},
	}

	res, err := importWikiCore(context.Background(), deps, importWikiInput{
		Project:       "silo2",
		Root:          legacy,
		IncludeReadme: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesFound != 2 {
		t.Fatalf("expected 2 files found, got %d", res.FilesFound)
	}
	matches, _ := filepath.Glob(filepath.Join(vaultDir, "Inbox/open/seed-*.md"))
	if len(matches) != 2 {
		t.Fatalf("expected 2 seeds, got %d (%v)", len(matches), matches)
	}
}

func TestImportWiki_LimitWorks(t *testing.T) {
	legacy := t.TempDir()
	for i := 0; i < 5; i++ {
		name := filepath.Join(legacy, "n"+string(rune('a'+i))+".md")
		if err := os.WriteFile(name, []byte("# T"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	vaultDir := t.TempDir()
	deps := importWikiDeps{
		Client:    engram.NewMockClient(),
		Generator: seed.NewMockGenerator(),
		Vault:     obsidian.NewVault(vaultDir),
		Stdout:    &bytes.Buffer{},
	}

	_, err := importWikiCore(context.Background(), deps, importWikiInput{
		Project: "silo2",
		Root:    legacy,
		Limit:   2,
	})
	if err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(vaultDir, "Inbox/open/seed-*.md"))
	if len(matches) != 2 {
		t.Fatalf("expected 2 seeds, got %d", len(matches))
	}
}

func TestImportWiki_DryRunDoesNotWrite(t *testing.T) {
	legacy := t.TempDir()
	if err := os.WriteFile(filepath.Join(legacy, "note.md"), []byte("# T"), 0o644); err != nil {
		t.Fatal(err)
	}
	vaultDir := t.TempDir()
	var out bytes.Buffer
	deps := importWikiDeps{
		Client:    engram.NewMockClient(),
		Generator: seed.NewMockGenerator(),
		Vault:     obsidian.NewVault(vaultDir),
		Stdout:    &out,
	}

	res, err := importWikiCore(context.Background(), deps, importWikiInput{
		Project: "silo2",
		Root:    legacy,
		DryRun:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.ObservationsSaved != 0 || res.SeedsWritten != 0 {
		t.Fatalf("dry-run should not write; got observations=%d seeds=%d", res.ObservationsSaved, res.SeedsWritten)
	}

	matches, _ := filepath.Glob(filepath.Join(vaultDir, "Inbox/open/seed-*.md"))
	if len(matches) != 0 {
		t.Fatalf("expected no seeds written, got %d", len(matches))
	}
}

func TestImportWiki_SeedIncludesSourcePath(t *testing.T) {
	legacy := t.TempDir()
	_ = os.MkdirAll(filepath.Join(legacy, "a"), 0o755)
	if err := os.WriteFile(filepath.Join(legacy, "a", "note.md"), []byte("# T\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	vaultDir := t.TempDir()
	deps := importWikiDeps{
		Client:    engram.NewMockClient(),
		Generator: seed.NewMockGenerator(),
		Vault:     obsidian.NewVault(vaultDir),
		Stdout:    &bytes.Buffer{},
	}

	_, err := importWikiCore(context.Background(), deps, importWikiInput{Project: "silo2", Root: legacy})
	if err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob(filepath.Join(vaultDir, "Inbox/open/seed-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 seed, got %d", len(matches))
	}
	b, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "## Source") || !strings.Contains(string(b), "Legacy path: a/note.md") {
		t.Fatalf("expected source section with legacy path, got:\n%s", string(b))
	}
}

func TestImportWiki_DeterministicAndIdempotentSeeds(t *testing.T) {
	legacy := t.TempDir()
	if err := os.WriteFile(filepath.Join(legacy, "note.md"), []byte("# T\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	vaultDir := t.TempDir()
	deps := importWikiDeps{
		Client:    engram.NewMockClient(),
		Generator: seed.NewMockGenerator(),
		Vault:     obsidian.NewVault(vaultDir),
		Stdout:    &bytes.Buffer{},
	}

	res1, err := importWikiCore(context.Background(), deps, importWikiInput{Project: "silo2", Root: legacy})
	if err != nil {
		t.Fatal(err)
	}
	if res1.SeedsWritten != 1 {
		t.Fatalf("expected 1 seed written, got %d", res1.SeedsWritten)
	}

	// New client (fresh process) would assign different observation IDs.
	deps.Client = engram.NewMockClient()
	res2, err := importWikiCore(context.Background(), deps, importWikiInput{Project: "silo2", Root: legacy})
	if err != nil {
		t.Fatal(err)
	}
	if res2.SeedsSkipped != 1 {
		t.Fatalf("expected seed to be skipped on rerun, got skipped=%d written=%d", res2.SeedsSkipped, res2.SeedsWritten)
	}

	matches, _ := filepath.Glob(filepath.Join(vaultDir, "Inbox/open/seed-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected 1 seed file total, got %d", len(matches))
	}
}
