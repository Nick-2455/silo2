package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/engram"
	"github.com/nicolasperalta/silo2/internal/obsidian"
	"github.com/nicolasperalta/silo2/internal/seed"
	"github.com/nicolasperalta/silo2/internal/synthesis"
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
		Client: client,
		Synth:  mockSynthesizerFromSeed(t, gen),
		Vault:  v,
		Stdout: &stdout,
		Stderr: &stderr,
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
		Client: client, Synth: mockSynthesizerFromSeed(t, gen), Vault: v,
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

func TestSaveCore_DoesNotPassCallerOwnedSourceIntoSynthesis(t *testing.T) {
	dir := t.TempDir()
	client := engram.NewMockClient()
	recorder := &recordingSynthesizer{proposal: synthesis.Proposal{
		ProposedSummary:  "summary",
		SuggestedThemes:  []string{"theme"},
		WhyItMightMatter: "matter",
	}}

	_, err := saveCore(context.Background(), saveDeps{
		Client: client,
		Synth:  recorder,
		Vault:  obsidian.NewVault(dir),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}, saveInput{
		Project:    "silo2",
		Text:       "raw content here",
		Why:        "Human why",
		SourceURL:  "https://example.com/post",
		SourceType: "article",
	})
	if err != nil {
		t.Fatalf("saveCore: %v", err)
	}
	if recorder.recorded.ContextHint != "silo save observation" {
		t.Fatalf("ContextHint = %q", recorder.recorded.ContextHint)
	}
	if recorder.recorded.Title != "" {
		t.Fatalf("Title = %q, want empty", recorder.recorded.Title)
	}
	if recorder.recorded.Content != "raw content here" {
		t.Fatalf("Content = %q", recorder.recorded.Content)
	}
	if strings.Contains(recorder.recorded.Content, "https://example.com/post") ||
		strings.Contains(recorder.recorded.Content, "article") ||
		strings.Contains(recorder.recorded.ContextHint, "https://example.com/post") ||
		strings.Contains(recorder.recorded.ContextHint, "article") {
		t.Fatalf("caller-owned source leaked into synthesis input: %+v", recorder.recorded)
	}
	if strings.Contains(recorder.recorded.Title, "https://example.com/post") || strings.Contains(recorder.recorded.Title, "article") {
		t.Fatalf("caller-owned source leaked into synthesis title: %+v", recorder.recorded)
	}
}

func TestSaveCore_RejectsEmptyInput(t *testing.T) {
	_, err := saveCore(context.Background(), saveDeps{
		Client: engram.NewMockClient(),
		Synth:  mockSynthesizerFromSeed(t, seed.NewMockGenerator()),
		Vault:  obsidian.NewVault(t.TempDir()),
		Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
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
		Client: engram.NewMockClient(),
		Synth:  mockSynthesizerFromSeed(t, seed.NewMockGenerator()),
		Vault:  obsidian.NewVault(dir),
		Stdout: &stdout, Stderr: &bytes.Buffer{},
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
		Client: client, Synth: mockSynthesizerFromSeed(t, seed.NewMockGenerator()), Vault: v,
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

func TestSaveCore_DoesNotOverwriteExistingSeedWhenSourceMetadataMatches(t *testing.T) {
	dir := t.TempDir()
	deps := saveDeps{
		Client: fixedIDClient{id: "obs-fixed-1"},
		Synth:  mockSynthesizer{proposal: synthesis.Proposal{ProposedSummary: "summary", SuggestedThemes: []string{"theme"}, WhyItMightMatter: "matter"}},
		Vault:  obsidian.NewVault(dir),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	in := saveInput{
		Project:    "silo2",
		Text:       "same content",
		Why:        "same why",
		SourceURL:  "https://example.com/post",
		SourceType: "article",
	}

	first, err := saveCore(context.Background(), deps, in)
	if err != nil {
		t.Fatalf("first saveCore: %v", err)
	}
	seedPath := filepath.Join(dir, first.SeedPath)
	original := []byte("human edited seed")
	if err := os.WriteFile(seedPath, original, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	second, err := saveCore(context.Background(), deps, in)
	if err != nil {
		t.Fatalf("second saveCore: %v", err)
	}
	if first.SeedPath != second.SeedPath {
		t.Fatalf("seed path changed across identical save: %q vs %q", first.SeedPath, second.SeedPath)
	}
	body, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(body) != string(original) {
		t.Fatalf("existing seed was overwritten:\n%s", body)
	}
}

func TestSaveCore_SourceMetadataDoesNotAffectSeedPath(t *testing.T) {
	dir := t.TempDir()
	deps := saveDeps{
		Client: fixedIDClient{id: "obs-fixed-1"},
		Synth:  mockSynthesizer{proposal: synthesis.Proposal{ProposedSummary: "summary", SuggestedThemes: []string{"theme"}, WhyItMightMatter: "matter"}},
		Vault:  obsidian.NewVault(dir),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}

	base := saveInput{Project: "silo2", Text: "same content", Why: "same why"}
	firstInput := base
	firstInput.SourceURL = "https://example.com/article"
	firstInput.SourceType = "article"
	secondInput := base
	secondInput.SourceURL = "https://example.com/video"
	secondInput.SourceType = "video"

	first, err := saveCore(context.Background(), deps, firstInput)
	if err != nil {
		t.Fatalf("first saveCore: %v", err)
	}
	seedPath := filepath.Join(dir, first.SeedPath)
	original, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("ReadFile first seed: %v", err)
	}

	second, err := saveCore(context.Background(), deps, secondInput)
	if err != nil {
		t.Fatalf("second saveCore: %v", err)
	}
	if first.SeedPath != second.SeedPath {
		t.Fatalf("source metadata changed seed path: %q vs %q", first.SeedPath, second.SeedPath)
	}
	body, err := os.ReadFile(seedPath)
	if err != nil {
		t.Fatalf("ReadFile second seed: %v", err)
	}
	if string(body) != string(original) {
		t.Fatalf("different source metadata overwrote existing seed:\n%s", body)
	}
}

func TestParseSaveArgs_FlagAfterText(t *testing.T) {
	// The natural CLI form is text first, flags after. Pin the parser
	// so this never regresses to Go's default flag-then-positional rule.
	why, project, _, _, pos, err := parseSaveArgs([]string{"MVVM-C navigation insight", "--why", "I keep forgetting"})
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

func TestParseSaveArgs_SourceFlags(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantWhy        string
		wantProject    string
		wantSourceURL  string
		wantSourceType string
		wantPositional []string
	}{
		{
			name:           "source after text with default type",
			args:           []string{"hello", "--source", "https://example.com/post"},
			wantSourceURL:  "https://example.com/post",
			wantSourceType: "link",
			wantPositional: []string{"hello"},
		},
		{
			name:           "source before text equals form",
			args:           []string{"--source=https://example.com/post", "--source-type=article", "hello"},
			wantSourceURL:  "https://example.com/post",
			wantSourceType: "article",
			wantPositional: []string{"hello"},
		},
		{
			name:           "source mixed with why and project",
			args:           []string{"--project", "x", "hello", "--why", "reason", "--source-type", "video", "--source", "https://example.com/watch"},
			wantWhy:        "reason",
			wantProject:    "x",
			wantSourceURL:  "https://example.com/watch",
			wantSourceType: "video",
			wantPositional: []string{"hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			why, project, sourceURL, sourceType, pos, err := parseSaveArgs(tt.args)
			if err != nil {
				t.Fatalf("parseSaveArgs: %v", err)
			}
			if why != tt.wantWhy || project != tt.wantProject || sourceURL != tt.wantSourceURL || sourceType != tt.wantSourceType {
				t.Fatalf("got why=%q project=%q sourceURL=%q sourceType=%q", why, project, sourceURL, sourceType)
			}
			if strings.Join(pos, "|") != strings.Join(tt.wantPositional, "|") {
				t.Fatalf("positional = %v, want %v", pos, tt.wantPositional)
			}
		})
	}
}

func TestParseSaveArgs_InvalidSourceTypeRejected(t *testing.T) {
	_, _, _, _, _, err := parseSaveArgs([]string{"hello", "--source", "https://example.com/post", "--source-type", "podcast"})
	if err == nil {
		t.Fatal("expected invalid source-type error")
	}
	if !strings.Contains(err.Error(), "invalid source-type") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "podcast") {
		t.Fatalf("error should mention invalid value, got: %v", err)
	}
	if !strings.Contains(err.Error(), "article") || !strings.Contains(err.Error(), "link") {
		t.Fatalf("error should mention allowed values, got: %v", err)
	}
}

func TestParseSaveArgs_SourceTypeRequiresSource(t *testing.T) {
	_, _, _, _, _, err := parseSaveArgs([]string{"hello", "--source-type", "article"})
	if err == nil {
		t.Fatal("expected --source-type without --source to error")
	}
	if !strings.Contains(err.Error(), "--source-type requires --source") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseSaveArgs_FlagBeforeText(t *testing.T) {
	why, _, _, _, pos, err := parseSaveArgs([]string{"--why", "reason", "hello"})
	if err != nil {
		t.Fatal(err)
	}
	if why != "reason" || len(pos) != 1 || pos[0] != "hello" {
		t.Errorf("why=%q pos=%v", why, pos)
	}
}

func TestParseSaveArgs_EqualsForm(t *testing.T) {
	why, project, _, _, pos, err := parseSaveArgs([]string{"--why=reason", "--project=x", "text"})
	if err != nil {
		t.Fatal(err)
	}
	if why != "reason" || project != "x" || pos[0] != "text" {
		t.Errorf("why=%q project=%q pos=%v", why, project, pos)
	}
}

func TestParseSaveArgs_UnknownFlagRejected(t *testing.T) {
	if _, _, _, _, _, err := parseSaveArgs([]string{"--wy", "typo", "text"}); err == nil {
		t.Error("expected error on unknown flag")
	}
}

func TestParseSaveArgs_DoubleDashStopsParsing(t *testing.T) {
	// Allow capturing literal text that starts with "--".
	_, _, _, _, pos, err := parseSaveArgs([]string{"--", "--literal-text", "more"})
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
		Client: unsupportedClient{},
		Synth:  mockSynthesizerFromSeed(t, seed.NewMockGenerator()),
		Vault:  v,
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}, saveInput{Project: "silo2", Text: "hello"})

	if err == nil {
		t.Fatal("expected an error from unsupported backend")
	}
	if !strings.Contains(err.Error(), "engram_endpoint") {
		t.Errorf("expected hint mentioning engram_endpoint, got: %v", err)
	}
}

func TestSaveCore_FallsBackWhenSynthesisFailsAndKeepsHumanBoundaries(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	res, err := saveCore(context.Background(), saveDeps{
		Client: engram.NewMockClient(),
		Synth:  errSynthesizer{err: context.DeadlineExceeded},
		Vault:  obsidian.NewVault(dir),
		Stdout: &stdout,
		Stderr: &stderr,
	}, saveInput{
		Project: "silo2",
		Text:    "Captured title from input",
		Why:     "Human reason survives",
	})
	if err != nil {
		t.Fatalf("saveCore: %v", err)
	}
	if res.ObservationID == "" || res.SeedPath == "" {
		t.Fatalf("expected capture and fallback seed, got %+v", res)
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "Inbox/open/seed-*.md"))
	if len(matches) != 1 {
		t.Fatalf("expected one fallback seed, got %v", matches)
	}
	body, readErr := os.ReadFile(matches[0])
	if readErr != nil {
		t.Fatal(readErr)
	}
	md := string(body)
	if !strings.Contains(md, "# Captured title from input") {
		t.Fatalf("seed title should come from observation input, got:\n%s", md)
	}
	if !strings.Contains(md, "## Capture Why") || !strings.Contains(md, "Human reason survives") {
		t.Fatalf("expected human why section in fallback seed, got:\n%s", md)
	}
	if !strings.Contains(md, "## Human Notes\n\nTODO.") {
		t.Fatalf("expected untouched Human Notes placeholder, got:\n%s", md)
	}
	if !strings.Contains(stderr.String(), "fallback") {
		t.Fatalf("expected fallback warning, got: %s", stderr.String())
	}
	if _, statErr := os.Stat(filepath.Join(dir, "Curated")); !os.IsNotExist(statErr) {
		t.Fatalf("save should not create Curated output, stat err=%v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "Outputs")); !os.IsNotExist(statErr) {
		t.Fatalf("save should not create Outputs, stat err=%v", statErr)
	}
}

func TestSynthesizeWithFallback_RespectsConfiguredTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{name: "two seconds", timeout: 2 * time.Second},
		{name: "thirty seconds", timeout: 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synth := &deadlineSynthesizer{}
			started := time.Now()

			_, err := synthesizeWithFallback(context.Background(), synth, synthesis.Source{Content: "x"}, tt.timeout)
			if err != nil {
				t.Fatalf("synthesizeWithFallback() error = %v", err)
			}

			assertDeadlineNear(t, synth.deadline, started.Add(tt.timeout), 100*time.Millisecond)
		})
	}
}

func TestSynthesizeWithFallback_ZeroOrNegativeUsesDefault(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{name: "zero", timeout: 0},
		{name: "negative", timeout: -time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synth := &deadlineSynthesizer{}
			started := time.Now()

			_, err := synthesizeWithFallback(context.Background(), synth, synthesis.Source{Content: "x"}, tt.timeout)
			if err != nil {
				t.Fatalf("synthesizeWithFallback() error = %v", err)
			}

			assertDeadlineNear(t, synth.deadline, started.Add(config.DefaultLLMTimeout), 100*time.Millisecond)
		})
	}
}

func TestSynthesizeWithFallback_FallbackStillFiresOnTimeout(t *testing.T) {
	src := synthesis.Source{Title: "Captured title", Content: "Body text"}
	synth := &deadlineSynthesizer{block: true}

	proposal, err := synthesizeWithFallback(context.Background(), synth, src, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error returned alongside fallback proposal")
	}

	want, fallbackErr := synthesis.NewFallback().Synthesize(context.Background(), src)
	if fallbackErr != nil {
		t.Fatalf("fallback synthesize: %v", fallbackErr)
	}
	if proposal.ProposedSummary != want.ProposedSummary {
		t.Fatalf("ProposedSummary = %q, want %q", proposal.ProposedSummary, want.ProposedSummary)
	}
	if strings.Join(proposal.SuggestedThemes, ",") != strings.Join(want.SuggestedThemes, ",") {
		t.Fatalf("SuggestedThemes = %v, want %v", proposal.SuggestedThemes, want.SuggestedThemes)
	}
	if proposal.WhyItMightMatter != want.WhyItMightMatter {
		t.Fatalf("WhyItMightMatter = %q, want %q", proposal.WhyItMightMatter, want.WhyItMightMatter)
	}
}

type mockSynthesizer struct {
	proposal synthesis.Proposal
	err      error
}

func (m mockSynthesizer) Synthesize(context.Context, synthesis.Source) (synthesis.Proposal, error) {
	if m.err != nil {
		return synthesis.Proposal{}, m.err
	}
	return m.proposal, nil
}

type recordingSynthesizer struct {
	recorded synthesis.Source
	proposal synthesis.Proposal
}

func (r *recordingSynthesizer) Synthesize(_ context.Context, src synthesis.Source) (synthesis.Proposal, error) {
	r.recorded = src
	return r.proposal, nil
}

type errSynthesizer struct{ err error }

func (e errSynthesizer) Synthesize(context.Context, synthesis.Source) (synthesis.Proposal, error) {
	return synthesis.Proposal{}, e.err
}

type deadlineSynthesizer struct {
	deadline time.Time
	block    bool
}

func (d *deadlineSynthesizer) Synthesize(ctx context.Context, _ synthesis.Source) (synthesis.Proposal, error) {
	if deadline, ok := ctx.Deadline(); ok {
		d.deadline = deadline
	}
	if d.block {
		<-ctx.Done()
		return synthesis.Proposal{}, ctx.Err()
	}
	return synthesis.Proposal{ProposedSummary: "x", SuggestedThemes: []string{"t"}, WhyItMightMatter: "y"}, nil
}

func assertDeadlineNear(t *testing.T, got, want time.Time, tolerance time.Duration) {
	t.Helper()
	if got.IsZero() {
		t.Fatal("expected synthesizer context deadline, got zero time")
	}
	delta := got.Sub(want)
	if delta < 0 {
		delta = -delta
	}
	if delta > tolerance {
		t.Fatalf("deadline = %v, want within %v of %v (delta %v)", got, tolerance, want, delta)
	}
}

func mockSynthesizerFromSeed(t *testing.T, gen *seed.MockGenerator) mockSynthesizer {
	t.Helper()
	seedValue, err := gen.Generate(engram.Observation{ID: "obs-seed", Title: "T", Content: "Body"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return mockSynthesizer{proposal: synthesis.Proposal{
		ProposedSummary:  seedValue.ProposedSummary,
		SuggestedThemes:  seedValue.SuggestedThemes,
		WhyItMightMatter: seedValue.WhyItMightMatter,
	}}
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

type fixedIDClient struct{ id string }

func (c fixedIDClient) Search(context.Context, string) ([]engram.Observation, error) {
	return nil, nil
}

func (c fixedIDClient) Context(context.Context, string) ([]engram.Observation, error) {
	return nil, nil
}

func (c fixedIDClient) Save(_ context.Context, _ engram.Observation) (string, error) {
	return c.id, nil
}
