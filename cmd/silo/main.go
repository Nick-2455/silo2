package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/engram"
	"github.com/nicolasperalta/silo2/internal/identity"
	"github.com/nicolasperalta/silo2/internal/markdown"
	"github.com/nicolasperalta/silo2/internal/obsidian"
)

func main() {
	if len(os.Args) < 2 {
		printHelp(os.Stdout)
		os.Exit(0)
	}

	cmd := os.Args[1]
	rest := os.Args[2:]

	switch cmd {
	case "help", "-h", "--help":
		printHelp(os.Stdout)
		return
	case "init":
		if err := runInit(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	case "sync":
		if err := runSync(rest); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	case "profile":
		if err := runProfile(rest); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	case "curate":
		if err := runCurate(rest); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	case "outputs":
		if err := runOutputs(rest); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printHelp(os.Stderr)
		os.Exit(2)
	}
}

func printHelp(w *os.File) {
	fmt.Fprint(w, `silo2 (MVP)

Usage:
  silo <command> [flags]

Commands:
  init     Create ./silo.config.json (idempotent)
  sync     Read observations from Engram and write Raw/Observations notes
  profile  Generate identity Markdown notes into the configured vault
  curate   Seed human-editable notes under Curated/ (never overwrites)
  outputs  Seed professional outputs (CV / LinkedIn / Bio) under Outputs/
  help     Print this help

Flags:
  --project <name>   Engram project to read from.
                     Precedence: --project flag > config.project > "silo2" (dev default).

Config:
  Uses ./silo.config.json for MVP simplicity. Default vault path is ./vault.
  Optional field "project" selects the Engram project. If absent, the CLI
  falls back to the temporary default "silo2" (intended for local dev only).
`)
}

func runInit() error {
	if config.Exists() {
		fmt.Printf("config already exists at %s\n", config.Path())
		return nil
	}
	cfg := config.Default()
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("created config at %s\n", config.Path())
	return nil
}

// parseProjectFlag builds a per-subcommand flag set with --project. Each
// subcommand owns its own flag set so users can write the natural form:
//   silo sync --project silo2
//   silo profile --project silo2
func parseProjectFlag(cmd string, args []string) (string, error) {
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	projectFlag := fs.String("project", "", "Engram project (overrides config.project)")
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	return *projectFlag, nil
}

func runSync(args []string) error {
	projectFlag, err := parseProjectFlag("sync", args)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	project := resolveProject(projectFlag, cfg.Project)
	fmt.Printf("Using Engram project: %s\n", project)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := engram.NewClient(cfg)
	obs, err := client.Context(ctx, project)
	if err != nil {
		return fmt.Errorf("engram context: %w", err)
	}

	notes, err := markdown.RenderObservations(obs)
	if err != nil {
		return fmt.Errorf("render observations: %w", err)
	}

	v := obsidian.NewVault(cfg.VaultPath)
	if err := v.EnsureDir(); err != nil {
		return err
	}

	const subdir = "Raw/Observations"
	for name, content := range notes {
		if err := v.WriteNoteAt(subdir, name, content); err != nil {
			return fmt.Errorf("write %s/%s: %w", subdir, name, err)
		}
	}

	fmt.Printf("vault: %s\n", cfg.VaultPath)
	if cfg.EngramEndpoint == "" {
		fmt.Println("engram: mock client (no endpoint configured)")
	} else {
		fmt.Printf("engram: %s\n", cfg.EngramEndpoint)
	}
	fmt.Printf("observations read: %d\n", len(obs))
	fmt.Printf("notes written: %d\n", len(notes))
	fmt.Printf("destination: %s/%s\n", cfg.VaultPath, subdir)
	return nil
}

func runProfile(args []string) error {
	projectFlag, err := parseProjectFlag("profile", args)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.VaultPath == "" {
		return errors.New("vault_path is empty")
	}
	project := resolveProject(projectFlag, cfg.Project)
	fmt.Printf("Using Engram project: %s\n", project)

	client := engram.NewClient(cfg)
	src, err := loadIdentitySource(context.Background(), cfg, client, project)
	if err != nil {
		return err
	}
	fmt.Println(src.CLILabel)

	ident, err := identity.BuildIdentity(src.Observations, cfg)
	if err != nil {
		return err
	}

	notes, err := markdown.Render(ident)
	if err != nil {
		return err
	}

	v := obsidian.NewVault(cfg.VaultPath)
	if err := v.EnsureDir(); err != nil {
		return err
	}

	for name, content := range notes {
		if err := v.WriteNote(name, content); err != nil {
			return err
		}
		fmt.Printf("wrote %s/%s\n", cfg.VaultPath, name)
	}

	return nil
}

// plural returns "s" unless n == 1. Tiny helper to keep CLI output grammatical.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// runCurate seeds human-editable notes under <vault>/Curated/<bucket>/.
//
// Idempotency contract: rendering is deterministic, and we only write a
// file when it does not yet exist. So:
//   - First run: writes N seed files.
//   - Subsequent runs without code changes: writes 0 (everything Skipped).
//   - User edits in Curated/ survive forever; Silo never touches them.
//
// This command intentionally does NOT touch Raw/Observations. Run
// `silo sync` for that. Run order does not matter for correctness, but
// running sync first means the Raw/Observations/*.md links inside
// curated notes resolve in Obsidian on day one.
func runCurate(args []string) error {
	projectFlag, err := parseProjectFlag("curate", args)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.VaultPath == "" {
		return errors.New("vault_path is empty")
	}
	project := resolveProject(projectFlag, cfg.Project)
	fmt.Printf("Using Engram project: %s\n", project)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client := engram.NewClient(cfg)
	obs, err := client.Context(ctx, project)
	if err != nil {
		return fmt.Errorf("engram context: %w", err)
	}

	notes, err := markdown.RenderCurated(obs)
	if err != nil {
		return fmt.Errorf("render curated: %w", err)
	}

	v := obsidian.NewVault(cfg.VaultPath)
	if err := v.EnsureDir(); err != nil {
		return err
	}

	const root = "Curated"
	// Write in sorted order so the CLI output is stable across runs.
	paths := make([]string, 0, len(notes))
	for p := range notes {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	written, skipped := 0, 0
	for _, rel := range paths {
		content := notes[rel]
		// rel uses forward slashes (e.g. "Architecture/silo-design.md").
		// Split into <subdir-under-Curated>/<filename>.
		subdir := root
		name := rel
		if i := strings.LastIndex(rel, "/"); i >= 0 {
			subdir = root + "/" + rel[:i]
			name = rel[i+1:]
		}
		res, err := v.WriteNoteIfAbsent(subdir, name, content)
		if err != nil {
			return fmt.Errorf("write %s/%s: %w", subdir, name, err)
		}
		if res == obsidian.Written {
			written++
			fmt.Printf("wrote    %s/%s/%s\n", cfg.VaultPath, subdir, name)
		} else {
			skipped++
			fmt.Printf("skipped  %s/%s/%s (already exists)\n", cfg.VaultPath, subdir, name)
		}
	}

	fmt.Printf("\nobservations read: %d\n", len(obs))
	fmt.Printf("curated notes written: %d\n", written)
	fmt.Printf("curated notes skipped (kept human edits): %d\n", skipped)
	fmt.Printf("destination: %s/%s\n", cfg.VaultPath, root)
	return nil
}

// runOutputs seeds professional outputs (CV, LinkedIn, ProfessionalBio)
// under <vault>/Outputs/.
//
// Source selection mirrors `silo profile` exactly via loadIdentitySource:
// Curated wins when useful, Engram is the fallback. The same Identity
// produced for the profile drives the output templates.
//
// Idempotency contract (identical to `silo curate`):
//   - First run: writes the 3 seed files.
//   - Subsequent runs: writes 0, skips 3. Human edits never get clobbered.
//   - Rendering itself is deterministic on the same Identity input.
//
// The frontmatter of each generated file records `source: curated` or
// `source: raw/engram` so a human opening CV.md a week later can tell
// which layer fed the seed.
func runOutputs(args []string) error {
	projectFlag, err := parseProjectFlag("outputs", args)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.VaultPath == "" {
		return errors.New("vault_path is empty")
	}
	project := resolveProject(projectFlag, cfg.Project)
	fmt.Printf("Using Engram project: %s\n", project)

	client := engram.NewClient(cfg)
	src, err := loadIdentitySource(context.Background(), cfg, client, project)
	if err != nil {
		return err
	}
	fmt.Println(src.CLILabel)

	ident, err := identity.BuildIdentity(src.Observations, cfg)
	if err != nil {
		return err
	}

	notes, err := markdown.RenderProfessionalOutputs(ident, src.Origin)
	if err != nil {
		return fmt.Errorf("render outputs: %w", err)
	}

	v := obsidian.NewVault(cfg.VaultPath)
	if err := v.EnsureDir(); err != nil {
		return err
	}

	const subdir = "Outputs"
	// Stable filename order so CLI output diffs cleanly across runs.
	names := make([]string, 0, len(notes))
	for n := range notes {
		names = append(names, n)
	}
	sort.Strings(names)

	written, skipped := 0, 0
	for _, name := range names {
		res, err := v.WriteNoteIfAbsent(subdir, name, notes[name])
		if err != nil {
			return fmt.Errorf("write %s/%s: %w", subdir, name, err)
		}
		if res == obsidian.Written {
			written++
			fmt.Printf("wrote    %s/%s/%s\n", cfg.VaultPath, subdir, name)
		} else {
			skipped++
			fmt.Printf("skipped  %s/%s/%s (already exists)\n", cfg.VaultPath, subdir, name)
		}
	}

	fmt.Printf("\noutputs written: %d\n", written)
	fmt.Printf("outputs skipped (kept human edits): %d\n", skipped)
	fmt.Printf("destination: %s/%s\n", cfg.VaultPath, subdir)
	return nil
}
