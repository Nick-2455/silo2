package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/engram"
	"github.com/nicolasperalta/silo2/internal/obsidian"
	"github.com/nicolasperalta/silo2/internal/seed"
)

// `silo import-wiki <path>` imports a legacy Obsidian wiki folder as reviewable
// Inbox/open seeds.
//
// Constraints (pinned by tests):
//   - NEVER modifies the legacy source vault.
//   - NEVER writes Curated/ directly.
//   - Deterministic + idempotent rendering (WriteNoteIfAbsent).
//   - Path is a weak signal only: stored in the seed body under "## Source".
//   - No LLM: uses the deterministic mock generator.
//   - Imports wiki/ only (raw/ is out of scope).

type importWikiDeps struct {
	Client    engram.Client
	Generator seed.Generator
	Vault     *obsidian.Vault
	Stdout    io.Writer
}

type importWikiInput struct {
	Project       string
	Root          string
	Limit         int
	IncludeReadme bool
	DryRun        bool
}

type importWikiResult struct {
	FilesFound        int
	FilesSkipped      int
	ObservationsSaved int
	SeedsWritten      int
	SeedsSkipped      int
	Destination       string
}

func runImportWiki(args []string) error {
	limitVal, includeReadmeVal, dryRunVal, projectVal, positional, err := parseImportWikiArgs(args)
	if err != nil {
		return err
	}
	if len(positional) != 1 {
		return errors.New("silo import-wiki: missing <path>\n\nUsage: silo import-wiki <path> [--limit N] [--include-readme] [--dry-run] [--project <name>]")
	}
	root := positional[0]
	if strings.TrimSpace(root) == "" {
		return errors.New("silo import-wiki: path is empty")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.VaultPath == "" {
		return errors.New("vault_path is empty in config")
	}
	project := resolveProject(projectVal, cfg.Project)

	deps := importWikiDeps{
		Client:    engram.NewClient(cfg),
		Generator: seed.NewMockGenerator(),
		Vault:     obsidian.NewVault(cfg.VaultPath),
		Stdout:    os.Stdout,
	}
	_, err = importWikiCore(context.Background(), deps, importWikiInput{
		Project:       project,
		Root:          root,
		Limit:         limitVal,
		IncludeReadme: includeReadmeVal,
		DryRun:        dryRunVal,
	})
	return err
}

// parseImportWikiArgs handles flags anywhere in the arg list (before or after
// the positional path), matching `silo save` behavior.
//
// Supported:
//
//	--limit N | --limit=N
//	--include-readme
//	--dry-run
//	--project <name> | --project=<name>
func parseImportWikiArgs(args []string) (limit int, includeReadme bool, dryRun bool, project string, positional []string, err error) {
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--limit":
			if i+1 >= len(args) {
				return 0, false, false, "", nil, errors.New("--limit requires a value")
			}
			v := args[i+1]
			n, convErr := parseNonNegativeInt(v)
			if convErr != nil {
				return 0, false, false, "", nil, fmt.Errorf("invalid --limit %q: %w", v, convErr)
			}
			limit = n
			i += 2
		case strings.HasPrefix(a, "--limit="):
			v := strings.TrimPrefix(a, "--limit=")
			n, convErr := parseNonNegativeInt(v)
			if convErr != nil {
				return 0, false, false, "", nil, fmt.Errorf("invalid --limit %q: %w", v, convErr)
			}
			limit = n
			i++
		case a == "--include-readme":
			includeReadme = true
			i++
		case a == "--dry-run":
			dryRun = true
			i++
		case a == "--project":
			if i+1 >= len(args) {
				return 0, false, false, "", nil, errors.New("--project requires a value")
			}
			project = args[i+1]
			i += 2
		case strings.HasPrefix(a, "--project="):
			project = strings.TrimPrefix(a, "--project=")
			i++
		case a == "--":
			positional = append(positional, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "--"):
			return 0, false, false, "", nil, fmt.Errorf("unknown flag: %s", a)
		default:
			positional = append(positional, a)
			i++
		}
	}
	return limit, includeReadme, dryRun, project, positional, nil
}

func parseNonNegativeInt(s string) (int, error) {
	if strings.TrimSpace(s) == "" {
		return 0, errors.New("empty")
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, errors.New("must be >= 0")
	}
	return n, nil
}

func importWikiCore(ctx context.Context, deps importWikiDeps, in importWikiInput) (importWikiResult, error) {
	if deps.Client == nil || deps.Generator == nil || deps.Vault == nil || deps.Stdout == nil {
		return importWikiResult{}, errors.New("import-wiki: missing dependencies")
	}
	if strings.TrimSpace(in.Project) == "" {
		return importWikiResult{}, errors.New("import-wiki: project is empty")
	}
	if strings.TrimSpace(in.Root) == "" {
		return importWikiResult{}, errors.New("import-wiki: root path is empty")
	}
	if in.Limit < 0 {
		return importWikiResult{}, errors.New("import-wiki: limit must be >= 0")
	}

	absRoot, err := filepath.Abs(in.Root)
	if err != nil {
		return importWikiResult{}, err
	}
	st, err := os.Stat(absRoot)
	if err != nil {
		return importWikiResult{}, err
	}
	if !st.IsDir() {
		return importWikiResult{}, fmt.Errorf("import-wiki: path is not a directory: %s", absRoot)
	}

	paths, skipped, err := collectMarkdownFiles(absRoot, in.IncludeReadme)
	if err != nil {
		return importWikiResult{}, err
	}
	// Deterministic ordering.
	sort.Strings(paths)

	res := importWikiResult{
		FilesFound:   len(paths),
		FilesSkipped: skipped,
		Destination:  filepath.Join(deps.Vault.Path, "Inbox/open"),
	}

	toProcess := paths
	if in.Limit > 0 && len(toProcess) > in.Limit {
		toProcess = toProcess[:in.Limit]
	}

	if in.DryRun {
		fmt.Fprintf(deps.Stdout, "files found: %d\n", res.FilesFound)
		fmt.Fprintf(deps.Stdout, "files skipped: %d\n", res.FilesSkipped)
		fmt.Fprintf(deps.Stdout, "would import: %d\n", len(toProcess))
		for _, p := range toProcess {
			rel, _ := filepath.Rel(absRoot, p)
			fmt.Fprintf(deps.Stdout, "  %s\n", filepath.ToSlash(rel))
		}
		fmt.Fprintf(deps.Stdout, "destination: %s\n", res.Destination)
		return res, nil
	}

	if err := deps.Vault.EnsureDir(); err != nil {
		return importWikiResult{}, err
	}
	// Ensure Inbox/README.md exists (same as save).
	if _, err := deps.Vault.WriteNoteIfAbsent("Inbox", "README.md", inboxReadme); err != nil {
		// Non-fatal; seeds are still the main output.
	}

	for _, p := range toProcess {
		b, err := os.ReadFile(p)
		if err != nil {
			return importWikiResult{}, fmt.Errorf("read %s: %w", p, err)
		}
		content := string(b)

		rel, err := filepath.Rel(absRoot, p)
		if err != nil {
			return importWikiResult{}, err
		}
		rel = filepath.ToSlash(rel)

		title := titleFromMarkdownOrFilename(content, filepath.Base(p))

		obs := engram.Observation{
			Title:     title,
			Type:      "legacy_wiki",
			Content:   content,
			Project:   strings.TrimSpace(in.Project),
			CreatedAt: time.Now().UTC(),
		}
		id, err := deps.Client.Save(ctx, obs)
		if err != nil {
			if errors.Is(err, engram.ErrSaveUnsupported) {
				return importWikiResult{}, fmt.Errorf(
					"silo import-wiki is not yet wired to the HTTP Engram backend. "+
						"Clear engram_endpoint in silo.config.json to use the local mock for import, "+
						"or wait for the next release. (underlying: %w)", err)
			}
			return importWikiResult{}, fmt.Errorf("save observation for %s: %w", p, err)
		}
		res.ObservationsSaved++
		obs.ID = id

		s, err := deps.Generator.Generate(obs)
		if err != nil {
			return importWikiResult{}, fmt.Errorf("seed generate for %s: %w", p, err)
		}
		// Seed identity for imports must be deterministic across runs.
		// Engram observation IDs are not stable for this purpose (the import
		// will generate a new observation each run). Hash the stable inputs
		// (legacy relative path + content) to avoid duplicating seeds.
		s.ID = importSeedID(rel, content)
		s.LegacyPath = rel

		md, err := seed.Render(s)
		if err != nil {
			return importWikiResult{}, fmt.Errorf("seed render for %s: %w", p, err)
		}

		const inboxOpen = "Inbox/open"
		filename := s.ID + ".md"
		wr, err := deps.Vault.WriteNoteIfAbsent(inboxOpen, filename, md)
		if err != nil {
			return importWikiResult{}, fmt.Errorf("write seed for %s: %w", p, err)
		}
		switch wr {
		case obsidian.Written:
			res.SeedsWritten++
		case obsidian.Skipped:
			res.SeedsSkipped++
		}
	}

	fmt.Fprintf(deps.Stdout, "files found: %d\n", res.FilesFound)
	fmt.Fprintf(deps.Stdout, "files skipped: %d\n", res.FilesSkipped)
	fmt.Fprintf(deps.Stdout, "observations saved: %d\n", res.ObservationsSaved)
	fmt.Fprintf(deps.Stdout, "seeds written: %d\n", res.SeedsWritten)
	fmt.Fprintf(deps.Stdout, "seeds skipped: %d\n", res.SeedsSkipped)
	fmt.Fprintf(deps.Stdout, "destination: %s\n", res.Destination)
	return res, nil
}

func collectMarkdownFiles(root string, includeReadme bool) (paths []string, skipped int, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			skipped++
			return nil
		}
		if !includeReadme {
			if name == "README.md" || strings.EqualFold(name, "readme.md") {
				skipped++
				return nil
			}
		}
		paths = append(paths, path)
		return nil
	})
	return paths, skipped, err
}

func titleFromMarkdownOrFilename(md string, filename string) string {
	if h1 := firstH1(md); h1 != "" {
		return h1
	}
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	if strings.TrimSpace(base) == "" {
		return "Untitled"
	}
	return base
}

func firstH1(md string) string {
	sc := bufio.NewScanner(strings.NewReader(md))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		// Ignore empty lines.
		if line == "" {
			continue
		}
		// Basic ATX H1 only: "# Title". This is a weak signal; we do not
		// attempt full Markdown parsing.
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
		// Stop early if we hit non-empty content without an H1.
		break
	}
	return ""
}

// importSeedID returns a stable seed ID for a legacy wiki file.
// It intentionally does NOT depend on Engram observation IDs.
func importSeedID(relPath string, content string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(strings.TrimSpace(relPath)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(content))
	sum := h.Sum(nil)
	return "seed-" + hex.EncodeToString(sum[:4])
}
