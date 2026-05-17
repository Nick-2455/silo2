// Package curated reads the human-editable layer at vault/Curated/ and
// converts notes that actually contain human content into synthetic
// engram.Observation values, so the rest of the system (notably
// identity.BuildIdentity and markdown.Render) can consume them without
// changes.
//
// Design notes:
//
//   - This is a READ-ONLY package. It never writes to the vault. The
//     write-side policy (never overwrite human edits) lives in
//     obsidian.WriteNoteIfAbsent, which is the single boundary for that
//     guarantee. Keeping read and write apart prevents accidental drift.
//
//   - "Useful human content" is detected by stripping noise (frontmatter,
//     headings, the auto-generated Related Observations block, TODO
//     placeholders) and checking whether any prose remains. See isUseful
//     for the exact rules — they exist as their own function on purpose
//     so behavior is testable in isolation.
//
//   - Bucket READMEs are skipped wholesale: they are indices Silo seeds,
//     not content, and matching them as "useful" would create false
//     positives that hide the legitimate Raw fallback.
//
//   - The synthetic Observation.ID follows the format "curated:<relpath>"
//     so anything downstream (Evidence rows, logs, debug output) can
//     distinguish curated sources from Engram sources at a glance.
package curated

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nicolasperalta/silo2/internal/engram"
)

// CuratedRoot is the directory under the vault root that this package
// reads. Kept as a constant so tests, the CLI, and the writer agree on
// the same path without string drift.
const CuratedRoot = "Curated"

// CuratedSourcePrefix marks an Observation as originating from a curated
// note rather than from Engram. Stable on purpose: identity rendering
// keys off this exact prefix when labeling Evidence rows.
const CuratedSourcePrefix = "curated:"

// LoadCurated walks <vaultPath>/Curated/**/*.md, parses each note, and
// returns one synthetic engram.Observation per note that contains useful
// human content. Notes that are still pristine Silo seeds (TODO-only) or
// that are bucket READMEs are skipped.
//
// project is stamped into each synthetic observation so downstream
// pipelines that group/filter by project keep working unchanged.
//
// Returns an empty slice (no error) when the Curated/ directory does not
// exist yet — that is the expected state right after `silo init` and the
// signal callers use to fall back to Raw/Engram.
func LoadCurated(vaultPath, project string) ([]engram.Observation, error) {
	if strings.TrimSpace(vaultPath) == "" {
		return nil, errors.New("vault path is empty")
	}
	root := filepath.Join(vaultPath, CuratedRoot)

	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []engram.Observation{}, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("curated root is not a directory: " + root)
	}

	var out []engram.Observation
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		if strings.EqualFold(d.Name(), "README.md") {
			// Bucket index, not content.
			return nil
		}

		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		note := parseNote(string(raw))
		if !note.useful {
			return nil
		}

		rel, relErr := filepath.Rel(vaultPath, path)
		if relErr != nil {
			rel = path
		}
		// Always use forward slashes in IDs so they remain stable across
		// operating systems and survive textual diffs.
		relSlash := filepath.ToSlash(rel)

		out = append(out, engram.Observation{
			ID:      CuratedSourcePrefix + relSlash,
			Title:   note.title,
			Type:    "curated",
			Content: note.content,
			Project: project,
			// CreatedAt intentionally zero. Curated notes are human-curated
			// projections, not timestamped events. The renderer already
			// tolerates zero CreatedAt.
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Deterministic order by synthetic ID (which encodes the relative
	// path). This keeps Evidence sections stable across runs.
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// parsedNote is the in-memory view of one curated markdown file.
type parsedNote struct {
	title   string
	content string // body usable as identity signal (frontmatter + Related stripped, headings kept for context)
	useful  bool
}

// parseNote strips frontmatter, isolates the first H1 as title, removes
// the auto-generated "Related Observations" section, and decides whether
// any human-written prose remains.
//
// The function is pure on a string so it is easy to test independently
// of the filesystem.
func parseNote(raw string) parsedNote {
	body := stripFrontmatter(raw)
	title := firstH1(body)
	body = stripRelatedObservations(body)

	useful := hasUsefulProse(body)
	return parsedNote{
		title:   title,
		content: strings.TrimSpace(body),
		useful:  useful,
	}
}

// stripFrontmatter removes a YAML frontmatter block if present at the
// very top of the document (lines surrounded by --- markers). Anything
// else is returned unchanged.
func stripFrontmatter(s string) string {
	s = strings.TrimLeft(s, "\ufeff") // tolerate BOM
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return s
	}
	// Find the closing "---" on its own line.
	rest := s[4:]
	if idx := strings.Index(rest, "\n---"); idx >= 0 {
		after := rest[idx+4:]
		// Skip the newline that follows the closing marker, if any.
		after = strings.TrimPrefix(after, "\r")
		after = strings.TrimPrefix(after, "\n")
		return after
	}
	return s
}

// firstH1 returns the text of the first "# Title" line found, or "".
func firstH1(s string) string {
	for _, line := range strings.Split(s, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "# ") {
			return strings.TrimSpace(l[2:])
		}
	}
	return ""
}

// stripRelatedObservations removes the "## Related Observations" section
// (heading + everything until the next heading of equal or lower depth,
// or EOF). This section is fully auto-generated and must not count as
// human signal.
func stripRelatedObservations(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	inSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inSection {
			if isRelatedHeading(trimmed) {
				inSection = true
				continue
			}
			out = append(out, line)
			continue
		}
		// In section: bail out when we hit any new heading of depth 1 or 2.
		if strings.HasPrefix(trimmed, "# ") || strings.HasPrefix(trimmed, "## ") {
			inSection = false
			out = append(out, line)
		}
		// Otherwise drop the line.
	}
	return strings.Join(out, "\n")
}

func isRelatedHeading(trimmed string) bool {
	low := strings.ToLower(trimmed)
	return low == "## related observations" || strings.HasPrefix(low, "## related observations")
}

// hasUsefulProse returns true if the body (already stripped of
// frontmatter and Related Observations) contains at least one line that
// looks like real human prose. The rules:
//
//   - ignore blank lines
//   - ignore heading lines ("#", "##", ...)
//   - ignore pure TODO placeholders ("TODO", "TODO.", "TODO: ...", "todo - ...")
//   - ignore HTML-style comments
//   - everything else, including bullet list items and free-form text,
//     counts as useful
//
// Bullet lists count on purpose: a user who pastes "- shipped X, - led Y"
// is giving real signal even if they did not write paragraphs.
func hasUsefulProse(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		l := strings.TrimSpace(line)
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "#") {
			continue
		}
		if strings.HasPrefix(l, "<!--") {
			continue
		}
		if isTodoLine(l) {
			continue
		}
		// Bullet/numbered list markers stripped before the TODO check,
		// so a "- TODO" bullet still counts as placeholder.
		stripped := stripBulletPrefix(l)
		if stripped == "" || isTodoLine(stripped) {
			continue
		}
		return true
	}
	return false
}

// isTodoLine matches lines that are only a TODO placeholder. Examples:
//
//	TODO
//	TODO.
//	TODO:
//	TODO: write summary
//	todo - flesh this out
//
// Anything else returns false.
func isTodoLine(l string) bool {
	low := strings.ToLower(strings.TrimSpace(l))
	if low == "todo" || low == "todo." {
		return true
	}
	// "todo" followed by punctuation (":", ".", "-", " ") at position 4.
	if strings.HasPrefix(low, "todo") {
		rest := strings.TrimSpace(low[4:])
		if rest == "" {
			return true
		}
		switch rest[0] {
		case ':', '.', '-':
			return true
		}
	}
	return false
}

// stripBulletPrefix removes a leading "- ", "* ", "+ " or "N. " marker.
func stripBulletPrefix(l string) string {
	if len(l) >= 2 && (l[0] == '-' || l[0] == '*' || l[0] == '+') && l[1] == ' ' {
		return strings.TrimSpace(l[2:])
	}
	// "1." / "12." style numbered list.
	i := 0
	for i < len(l) && l[i] >= '0' && l[i] <= '9' {
		i++
	}
	if i > 0 && i < len(l) && l[i] == '.' {
		return strings.TrimSpace(l[i+1:])
	}
	return l
}
