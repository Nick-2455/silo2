package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/obsidian"
)

// `silo videos` generates a deterministic global list of video seeds.
//
// Why this exists:
//   - Users want a centralized "watch later" view.
//   - Curated notes are human-owned and must not be overwritten.
//   - Seeds are the system's capture surface for links; so we project them
//     into a generated Raw list that is safe to regenerate.

type videoSeed struct {
	RelPath   string // Inbox/open/seed-....md or Inbox/archive/...
	Title     string
	Status    string
	SourceURL string
}

func runVideos(args []string) error {
	fs := flag.NewFlagSet("videos", flag.ContinueOnError)
	projectFlag := fs.String("project", "", "Engram project (unused; kept for consistency)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_ = projectFlag // currently unused; list is derived from vault seeds.

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.VaultPath) == "" {
		return errors.New("vault_path is empty in config")
	}

	vaultPath := cfg.VaultPath
	items, err := scanVideoSeeds(vaultPath)
	if err != nil {
		return err
	}

	md := renderVideoWatchLater(items, time.Now().UTC())
	v := obsidian.NewVault(vaultPath)
	if err := v.EnsureDir(); err != nil {
		return err
	}
	const subdir = "Raw/Lists"
	const filename = "Videos - Watch Later.md"
	if err := v.WriteNoteAt(subdir, filename, md); err != nil {
		return fmt.Errorf("write %s/%s: %w", subdir, filename, err)
	}

	fmt.Printf("%s/%s/%s\n", vaultPath, subdir, filename)
	fmt.Printf("videos listed: %d\n", len(items))
	return nil
}

func scanVideoSeeds(vaultPath string) ([]videoSeed, error) {
	var out []videoSeed
	for _, sub := range []string{"Inbox/open", "Inbox/archive"} {
		dir := filepath.Join(vaultPath, filepath.FromSlash(sub))
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".md") {
				continue
			}
			if name == "README.md" || strings.EqualFold(name, "readme.md") {
				continue
			}
			full := filepath.Join(dir, name)
			s, ok := readVideoSeed(full)
			if !ok {
				continue
			}
			s.RelPath = filepath.ToSlash(filepath.Join(sub, name))
			out = append(out, s)
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		// Stable across runs: sort by URL then title then path.
		if out[i].SourceURL != out[j].SourceURL {
			return out[i].SourceURL < out[j].SourceURL
		}
		if out[i].Title != out[j].Title {
			return out[i].Title < out[j].Title
		}
		return out[i].RelPath < out[j].RelPath
	})
	return out, nil
}

// readVideoSeed reads a seed markdown file and returns a videoSeed when the
// seed contains a Sources block with "video: <url>".
func readVideoSeed(path string) (videoSeed, bool) {
	f, err := os.Open(path)
	if err != nil {
		return videoSeed{}, false
	}
	defer f.Close()

	var out videoSeed
	out.Status = "open" // default

	sc := bufio.NewScanner(f)
	// Allow long lines (URLs, etc). Default token limit is 64K.
	sc.Buffer(make([]byte, 1024), 512*1024)

	// Very small markdown parser:
	// - frontmatter: read status
	// - first H1: title
	// - under "## Sources": extract "- video: <url>"
	inFrontmatter := false
	seenFirstLine := false
	inSources := false
	for sc.Scan() {
		line := sc.Text()
		trim := strings.TrimSpace(line)

		if !seenFirstLine {
			seenFirstLine = true
			if trim == "---" {
				inFrontmatter = true
				continue
			}
		}

		if inFrontmatter {
			if trim == "---" {
				inFrontmatter = false
				continue
			}
			if k, v, ok := splitYAMLLine(trim); ok && strings.EqualFold(k, "status") {
				out.Status = strings.ToLower(strings.TrimSpace(v))
			}
			continue
		}

		if strings.HasPrefix(trim, "# ") && out.Title == "" {
			out.Title = strings.TrimSpace(strings.TrimPrefix(trim, "# "))
			continue
		}

		if trim == "## Sources" {
			inSources = true
			continue
		}
		if strings.HasPrefix(trim, "## ") && trim != "## Sources" {
			inSources = false
		}
		if !inSources {
			continue
		}

		// Expected line: "- video: https://..."
		if !strings.HasPrefix(trim, "-") {
			continue
		}
		item := strings.TrimSpace(strings.TrimPrefix(trim, "-"))
		if !strings.HasPrefix(strings.ToLower(item), "video:") {
			continue
		}
		url := strings.TrimSpace(item[len("video:"):])
		if url == "" {
			continue
		}
		out.SourceURL = url
		if out.Title == "" {
			out.Title = filepath.Base(path)
		}
		return out, true
	}
	return videoSeed{}, false
}

func splitYAMLLine(line string) (key, val string, ok bool) {
	i := strings.IndexByte(line, ':')
	if i <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
}

func renderVideoWatchLater(items []videoSeed, now time.Time) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("type: generated-list\n")
	b.WriteString("generated_by: silo\n")
	b.WriteString("generated_kind: watch-later-videos\n")
	b.WriteString("---\n\n")
	b.WriteString("# Videos - Watch Later\n\n")
	b.WriteString("This file is generated by `silo videos`. Do not edit by hand.\n\n")
	_ = now // reserved for future; we intentionally do not stamp time (idempotency).

	if len(items) == 0 {
		b.WriteString("(no video seeds found)\n")
		return b.String()
	}

	b.WriteString("## Items\n\n")
	for _, it := range items {
		// Keep it Obsidian-friendly and scan-friendly.
		b.WriteString("- [")
		b.WriteString(escapeBrackets(it.Title))
		b.WriteString("](")
		b.WriteString(it.SourceURL)
		b.WriteString(")")
		b.WriteString("  ")
		b.WriteString("(status: ")
		b.WriteString(it.Status)
		b.WriteString(", seed: ")
		b.WriteString(it.RelPath)
		b.WriteString(")\n")
	}
	return b.String()
}

func escapeBrackets(s string) string {
	// Obsidian markdown links use [](). Escape only the bracket characters.
	s = strings.ReplaceAll(s, "[", "\\[")
	s = strings.ReplaceAll(s, "]", "\\]")
	return s
}
