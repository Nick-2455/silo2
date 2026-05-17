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

// `silo save` end-to-end behavior. We drive the core function with an
// in-process MockClient + MockGenerator + tmp vault so the test reflects
// real wiring without spinning a process.

func TestSaveCore_WritesObservationAndSeed(t *testing.T) {
	dir := t.TempDir()
	v := obsidian.NewVault(dir)
	client := engram.NewMockClient()
	gen := seed.NewMockGenerator()

	var stdout, stderr bytes.Buffer
	res, err := saveCore(context.Background(), saveDeps{
		Client:    client,
		Generator: gen,
		Vault:     v,
		Stdout:    &stdout,
		Stderr:    &stderr,
	}, saveInput{
		Project: "silo2",
		Text:    "MVVM-C navigation insight",
	})
	if err != nil {
		t.Fatalf("saveCore: %v", err)
	}

	// 1. Observation was persisted to the (mock) backend.
	if res.ObservationID == "" {
		t.Error("expected an ObservationID, got empty")
	}

	// 2. A seed file landed in Inbox/open/.
	matches, _ := filepath.Glob(filepath.Join(dir, "Inbox/open/seed-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected exactly 1 seed file under Inbox/open/, got %d (%v)", len(matches), matches)
	}

	body, _ := os.ReadFile(matches[0])
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "type: seed") {
		t.Errorf("seed missing frontmatter type:\n%s", bodyStr)
	}
	if !strings.Contains(bodyStr, "status: open") {
		t.Errorf("seed missing status: open:\n%s", bodyStr)
	}
	if !strings.Contains(bodyStr, "MVVM-C navigation insight") {
		t.Errorf("seed body missing captured title:\n%s", bodyStr)
	}

	// 3. Stdout shows the contract: "Saved. Seed pending." first, then
	//    the seed path (so the user can open it).
	out := stdout.String()
	savedIdx := strings.Index(out, "Saved. Seed pending.")
	if savedIdx < 0 {
		t.Errorf("expected 'Saved. Seed pending.' in stdout, got:\n%s", out)
	}
}

func TestSaveCore_PassesWhyAsCaptureMetadata(t *testing.T) {
	// --why must reach Memory as Observation.Why, never merged into
	// Content. This pins the "Memory is sacred" rule from the design.
	dir := t.TempDir()
	v := obsidian.NewVault(dir)
	client := engram.NewMockClient()
	gen := seed.NewMockGenerator()

	_, err := saveCore(context.Background(), saveDeps{
		Client: client, Generator: gen, Vault: v,
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}, saveInput{
		Project: "silo2",
		Text:    "raw content here",
		Why:     "I keep forgetting this approach.",
	})
	if err != nil {
		t.Fatalf("saveCore: %v", err)
	}

	// Pull the observation back out of the mock store via Context().
	obs, err := client.Context(context.Background(), "silo2")
	if err != nil {
		t.Fatal(err)
	}
	var found *engram.Observation
	for i := range obs {
		if obs[i].Content == "raw content here" {
			found = &obs[i]
			break
		}
	}
	if found == nil {
		t.Fatal("captured observation not found in backend")
	}
	if found.Why != "I keep forgetting this approach." {
		t.Errorf("Why not persisted: %q", found.Why)
	}
	if strings.Contains(found.Content, "I keep forgetting") {
		t.Errorf("Why leaked into Content — Memory contaminated:\n%s", found.Content)
	}
}

func TestSaveCore_RejectsEmptyInput(t *testing.T) {
	_, err := saveCore(context.Background(), saveDeps{
		Client:    engram.NewMockClient(),
		Generator: seed.NewMockGenerator(),
		Vault:     obsidian.NewVault(t.TempDir()),
		Stdout:    &bytes.Buffer{}, Stderr: &bytes.Buffer{},
	}, saveInput{Project: "silo2", Text: "   "})
	if err == nil {
		t.Error("expected error on empty/whitespace input")
	}
}

func TestSaveCore_PrintsSavedBeforeSeedPath(t *testing.T) {
	// Contract: capture acknowledges instantly. Even if seed generation
	// later printed extra context, "Saved. Seed pending." must come first
	// so the user knows the observation is persisted.
	dir := t.TempDir()
	var stdout bytes.Buffer
	_, err := saveCore(context.Background(), saveDeps{
		Client:    engram.NewMockClient(),
		Generator: seed.NewMockGenerator(),
		Vault:     obsidian.NewVault(dir),
		Stdout:    &stdout, Stderr: &bytes.Buffer{},
	}, saveInput{Project: "silo2", Text: "hello"})
	if err != nil {
		t.Fatal(err)
	}

	out := stdout.String()
	savedIdx := strings.Index(out, "Saved. Seed pending.")
	seedIdx := strings.Index(out, "Inbox/open/seed-")
	if savedIdx < 0 || seedIdx < 0 {
		t.Fatalf("missing expected lines in stdout:\n%s", out)
	}
	if savedIdx > seedIdx {
		t.Errorf("'Saved.' must come before seed path; got order saved=%d seed=%d", savedIdx, seedIdx)
	}
}

func TestSaveCore_SeedFailureDoesNotFailCapture(t *testing.T) {
	// Spec: "capture never blocks on AI". If the seed cannot be written
	// (here: vault path made invalid), the Observation must still be
	// persisted and the command must exit successfully with a stderr
	// warning. Otherwise the human loses data on transient failures.
	client := engram.NewMockClient()

	// Vault pointed at a path under a file (impossible to mkdir into).
	bogusRoot := filepath.Join(t.TempDir(), "regular-file")
	if err := os.WriteFile(bogusRoot, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	v := obsidian.NewVault(bogusRoot) // mkdir under a file will fail

	var stderr bytes.Buffer
	res, err := saveCore(context.Background(), saveDeps{
		Client: client, Generator: seed.NewMockGenerator(), Vault: v,
		Stdout: &bytes.Buffer{}, Stderr: &stderr,
	}, saveInput{Project: "silo2", Text: "hello"})

	if err != nil {
		t.Fatalf("capture must succeed even if seed fails: %v", err)
	}
	if res.ObservationID == "" {
		t.Error("observation should be persisted even when seed fails")
	}
	if !strings.Contains(stderr.String(), "warning") {
		t.Errorf("expected a warning on stderr about seed failure, got:\n%s", stderr.String())
	}
}

func TestParseSaveArgs_FlagAfterText(t *testing.T) {
	// The natural CLI form is text first, flags after. Pin the parser
	// so this never regresses to Go's default flag-then-positional rule.
	why, project, pos, err := parseSaveArgs([]string{"MVVM-C navigation insight", "--why", "I keep forgetting"})
	if err != nil {
		t.Fatal(err)
	}
	if why != "I keep forgetting" {
		t.Errorf("why = %q", why)
	}
	if project != "" {
		t.Errorf("project = %q", project)
	}
	if len(pos) != 1 || pos[0] != "MVVM-C navigation insight" {
		t.Errorf("positional = %v", pos)
	}
}

func TestParseSaveArgs_FlagBeforeText(t *testing.T) {
	why, _, pos, err := parseSaveArgs([]string{"--why", "reason", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if why != "reason" || len(pos) != 1 || pos[0] != "hello" {
		t.Errorf("why=%q pos=%v", why, pos)
	}
}

func TestParseSaveArgs_EqualsForm(t *testing.T) {
	why, project, pos, err := parseSaveArgs([]string{"--why=reason", "--project=x", "text"})
	if err != nil {
		t.Fatal(err)
	}
	if why != "reason" || project != "x" || pos[0] != "text" {
		t.Errorf("why=%q project=%q pos=%v", why, project, pos)
	}
}

func TestParseSaveArgs_UnknownFlagRejected(t *testing.T) {
	if _, _, _, err := parseSaveArgs([]string{"--wy", "typo", "text"}); err == nil {
		t.Error("expected error on unknown flag")
	}
}

func TestParseSaveArgs_DoubleDashStopsParsing(t *testing.T) {
	// Allow capturing literal text that starts with "--".
	_, _, pos, err := parseSaveArgs([]string{"--", "--literal-text", "more"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pos) != 2 || pos[0] != "--literal-text" {
		t.Errorf("positional = %v", pos)
	}
}

func TestSaveCore_UnsupportedBackendIsFriendly(t *testing.T) {
	// Defensive: if a future backend signals ErrSaveUnsupported, the
	// command must surface a friendly hint rather than a generic error.
	// Driven via a stub so the test does not depend on which concrete
	// backend currently returns the sentinel.
	dir := t.TempDir()
	v := obsidian.NewVault(dir)

	_, err := saveCore(context.Background(), saveDeps{
		Client:    unsupportedClient{},
		Generator: seed.NewMockGenerator(),
		Vault:     v,
		Stdout:    &bytes.Buffer{},
		Stderr:    &bytes.Buffer{},
	}, saveInput{Project: "silo2", Text: "hello"})

	if err == nil {
		t.Fatal("expected an error from unsupported backend")
	}
	if !strings.Contains(err.Error(), "engram_endpoint") {
		t.Errorf("expected hint mentioning engram_endpoint, got: %v", err)
	}
}

// unsupportedClient lets us exercise the ErrSaveUnsupported branch of
// saveCore without depending on which production backend returns it.
type unsupportedClient struct{}

func (unsupportedClient) Search(_ context.Context, _ string) ([]engram.Observation, error) {
	return nil, nil
}
func (unsupportedClient) Context(_ context.Context, _ string) ([]engram.Observation, error) {
	return nil, nil
}
func (unsupportedClient) Save(_ context.Context, _ engram.Observation) (string, error) {
	return "", engram.ErrSaveUnsupported
}
