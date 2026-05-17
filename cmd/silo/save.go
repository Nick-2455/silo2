package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/engram"
	"github.com/nicolasperalta/silo2/internal/obsidian"
	"github.com/nicolasperalta/silo2/internal/seed"
	"github.com/nicolasperalta/silo2/internal/synthesis"
)

// `silo save` is the W1 capture verb.
//
// Contract:
//
//   - Synchronous path persists the Observation to Memory (Engram) and
//     immediately prints "Saved. Seed pending.". This must happen even
//     if the synthesis step later fails — capture must never block on AI.
//   - Asynchronous-feeling path then generates a Seed proposal and writes
//     it under <vault>/Inbox/open/<seed-id>.md via WriteNoteIfAbsent so
//     a re-run with identical input does not duplicate or overwrite.
//   - --why is metadata about WHY the capture happened. It flows through
//     Observation.Why and is rendered in a dedicated "Capture Why"
//     section of the seed, attributed to the human. It MUST NOT be
//     merged into Observation.Content (Memory is sacred).
//
// MVP scope: text input only. URL / PDF / file extraction is a separate
// change. Seed synthesis is optional and proposal-only; deterministic
// fallback keeps capture reliable when AI is disabled or fails.

// inboxReadme is written to vault/Inbox/README.md on the first capture
// so the directory is discoverable in Obsidian and the human knows what
// belongs here. WriteNoteIfAbsent protects later edits to this file.
const inboxReadme = `---
type: inbox-index
generated_by: silo
---

# Inbox

This is the **Seed Inbox** — where Silo drops AI-generated synthesis
proposals for you to triage.

## Layout

- ` + "`open/`" + ` — fresh seeds waiting for your attention.
- ` + "`archive/`" + ` — seeds you are done thinking about.

## How to triage

Open a seed in Obsidian. State lives in the frontmatter:

` + "```yaml" + `
status: open | deferred | discarded | approved
` + "```" + `

To defer or discard, edit the field. To get a seed out of the active
list once you are done with it, move the file to ` + "`archive/`" + `.

**Promotion to ` + "`Curated/`" + ` is a human act.** Silo never moves
seeds into Curated automatically. If a seed deserves to become part of
your curated knowledge, copy what matters into a Curated note by hand.

## Why so manual?

Memory is sacred. Synthesis is cheap. Identity is earned. The seed
inbox is the editorial gate between AI proposals and your knowledge.
`

// saveDeps groups injectable dependencies so saveCore is testable in
// isolation. The CLI wrapper (runSave) builds them from config; tests
// build them with in-memory fakes.
type saveDeps struct {
	Client  engram.Client
	Synth   synthesis.Synthesizer
	Vault   *obsidian.Vault
	Timeout time.Duration
	Stdout  io.Writer
	Stderr  io.Writer
}

type saveInput struct {
	Project    string
	Text       string
	Why        string
	SourceURL  string
	SourceType string
}

type saveResult struct {
	ObservationID string
	SeedPath      string // relative path under vault; empty if seed failed
}

// saveCore performs the capture. It returns an error only when the
// synchronous capture itself fails. Seed-generation errors are surfaced
// on stderr as warnings; the caller still exits 0 because the
// observation is safely persisted to Memory.
func saveCore(ctx context.Context, deps saveDeps, in saveInput) (saveResult, error) {
	text := strings.TrimSpace(in.Text)
	if text == "" {
		return saveResult{}, errors.New("save: text is empty")
	}
	if deps.Client == nil || deps.Synth == nil || deps.Vault == nil {
		return saveResult{}, errors.New("save: missing dependencies")
	}

	// --- 1. Synchronous: persist Observation to Memory. ---
	obs := engram.Observation{
		// ID is assigned by the backend. Memory owns identity.
		Title:     "",
		Type:      "capture",
		Content:   text, // raw, untouched
		Project:   strings.TrimSpace(in.Project),
		Why:       strings.TrimSpace(in.Why), // capture metadata
		CreatedAt: time.Now().UTC(),
	}
	id, err := deps.Client.Save(ctx, obs)
	if err != nil {
		if errors.Is(err, engram.ErrSaveUnsupported) {
			// Friendly hint: this is exactly the moment a user will hit
			// this in real life (config has engram_endpoint set).
			return saveResult{}, fmt.Errorf(
				"silo save is not yet wired to the HTTP Engram backend. "+
					"Clear engram_endpoint in silo.config.json to use the local mock for capture, "+
					"or wait for the next release. (underlying: %w)", err)
		}
		return saveResult{}, fmt.Errorf("save observation: %w", err)
	}
	obs.ID = id

	// Acknowledge immediately. This is the contract the human relies on.
	fmt.Fprintln(deps.Stdout, "Saved. Seed pending.")

	// --- 2. Best-effort: generate and write the Seed. ---
	res := saveResult{ObservationID: id}

	src := synthesis.Source{
		Title:       obs.Title,
		Content:     obs.Content,
		ContextHint: "silo save observation",
	}
	proposal, synthErr := synthesizeWithFallback(ctx, deps.Synth, src, deps.Timeout)
	if synthErr != nil {
		fmt.Fprintf(deps.Stderr, "warning: synthesis failed (%v); using deterministic fallback for observation %s\n", synthErr, id)
	}

	s, err := seed.BuildFromObservation(obs, proposal)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "warning: seed generation failed (%v); observation %s is safe\n", err, id)
		return res, nil
	}
	s.SourceURL = strings.TrimSpace(in.SourceURL)
	s.SourceType = strings.TrimSpace(in.SourceType)

	md, err := seed.Render(s)
	if err != nil {
		fmt.Fprintf(deps.Stderr, "warning: seed render failed (%v); observation %s is safe\n", err, id)
		return res, nil
	}

	const inboxOpen = "Inbox/open"
	filename := s.ID + ".md"
	if err := deps.Vault.EnsureDir(); err != nil {
		fmt.Fprintf(deps.Stderr, "warning: vault setup failed (%v); observation %s is safe\n", err, id)
		return res, nil
	}
	// Seed a README under Inbox/ on first save so the directory is
	// discoverable in Obsidian and the human knows what lives here.
	// WriteNoteIfAbsent protects any human edits to this file.
	if _, err := deps.Vault.WriteNoteIfAbsent("Inbox", "README.md", inboxReadme); err != nil {
		fmt.Fprintf(deps.Stderr, "warning: inbox README setup failed (%v); continuing\n", err)
	}
	if _, err := deps.Vault.WriteNoteIfAbsent(inboxOpen, filename, md); err != nil {
		fmt.Fprintf(deps.Stderr, "warning: seed write failed (%v); observation %s is safe\n", err, id)
		return res, nil
	}

	rel := inboxOpen + "/" + filename
	res.SeedPath = rel
	fmt.Fprintf(deps.Stdout, "%s/%s\n", deps.Vault.Path, rel)
	return res, nil
}

// runSave is the CLI entry point invoked by main.go.
func runSave(args []string) error {
	// Manual parse so flags can appear AFTER the text. The natural form
	//   silo save "MVVM-C insight" --why "I keep forgetting this"
	// must work the same as
	//   silo save --why "..." "MVVM-C insight"
	// Go's stdlib flag package stops at the first positional, which would
	// otherwise swallow trailing flags into the text.
	whyVal, projectVal, sourceURL, sourceType, positional, err := parseSaveArgs(args)
	if err != nil {
		return err
	}

	if len(positional) == 0 {
		return errors.New("silo save: missing input text\n\nUsage: silo save <text> [--why \"...\"] [--source \"https://...\"] [--source-type article|video|course|book|paper|link] [--project <name>]")
	}
	text := strings.Join(positional, " ")

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.VaultPath == "" {
		return errors.New("vault_path is empty in config")
	}
	project := resolveProject(projectVal, cfg.Project)

	deps := saveDeps{
		Client:  engram.NewClient(cfg),
		Synth:   synthesis.NewConfigured(cfg.LLMProvider, cfg.LLMModel, cfg.LLMAPIKey),
		Vault:   obsidian.NewVault(cfg.VaultPath),
		Timeout: cfg.SynthesisTimeout(),
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}

	_, err = saveCore(context.Background(), deps, saveInput{
		Project:    project,
		Text:       text,
		Why:        whyVal,
		SourceURL:  sourceURL,
		SourceType: sourceType,
	})
	return err
}

func synthesizeWithFallback(ctx context.Context, synth synthesis.Synthesizer, src synthesis.Source, timeout time.Duration) (synthesis.Proposal, error) {
	if timeout <= 0 {
		timeout = config.DefaultLLMTimeout
	}
	synthCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	proposal, err := synth.Synthesize(synthCtx, src)
	if err == nil {
		return proposal, nil
	}
	proposal, fallbackErr := synthesis.NewFallback().Synthesize(ctx, src)
	if fallbackErr != nil {
		return synthesis.Proposal{}, errors.Join(err, fallbackErr)
	}
	return proposal, err
}

// parseSaveArgs handles --why and --project anywhere in the argument list
// (before or after the positional text), in either `--key value` or
// `--key=value` form. Everything else becomes positional text.
//
// Unknown long flags are rejected loudly so a typo like "--wy" does not
// silently end up in the captured text.
func parseSaveArgs(args []string) (why, project, sourceURL, sourceType string, positional []string, err error) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--why":
			if i+1 >= len(args) {
				return "", "", "", "", nil, errors.New("--why requires a value")
			}
			why = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--why="):
			why = strings.TrimPrefix(a, "--why=")
			i++
		case a == "--project":
			if i+1 >= len(args) {
				return "", "", "", "", nil, errors.New("--project requires a value")
			}
			project = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--project="):
			project = strings.TrimPrefix(a, "--project=")
			i++
		case a == "--source":
			if i+1 >= len(args) {
				return "", "", "", "", nil, errors.New("--source requires a value")
			}
			sourceURL = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--source="):
			sourceURL = strings.TrimPrefix(a, "--source=")
			i++
		case a == "--source-type":
			if i+1 >= len(args) {
				return "", "", "", "", nil, errors.New("--source-type requires a value")
			}
			sourceType = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--source-type="):
			sourceType = strings.TrimPrefix(a, "--source-type=")
			i++
		case a == "--":
			// Treat everything after `--` as positional, no flag parsing.
			positional = append(positional, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "--"):
			return "", "", "", "", nil, fmt.Errorf("unknown flag: %s", a)
		default:
			positional = append(positional, a)
			i++
		}
	}

	sourceURL, sourceType, err = normalizeSourceArgs(sourceURL, sourceType)
	if err != nil {
		return "", "", "", "", nil, err
	}

	return why, project, sourceURL, sourceType, positional, nil
}

var allowedSourceTypes = map[string]struct{}{
	"article": {},
	"video":   {},
	"course":  {},
	"book":    {},
	"paper":   {},
	"link":    {},
}

const allowedSourceTypesList = "article, video, course, book, paper, link"

func normalizeSourceArgs(sourceURL, sourceType string) (string, string, error) {
	sourceURL = strings.TrimSpace(sourceURL)
	sourceType = strings.TrimSpace(sourceType)
	if sourceURL == "" {
		if sourceType != "" {
			return "", "", errors.New("--source-type requires --source")
		}
		return "", "", nil
	}
	if sourceType == "" {
		return sourceURL, "link", nil
	}
	if _, ok := allowedSourceTypes[sourceType]; !ok {
		return "", "", fmt.Errorf("invalid source-type %q (allowed: %s)", sourceType, allowedSourceTypesList)
	}
	return sourceURL, sourceType, nil
}
