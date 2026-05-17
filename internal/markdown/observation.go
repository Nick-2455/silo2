package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/nicolasperalta/silo2/internal/engram"
)

// maxSlugLen caps slug length to keep filenames sane across filesystems.
const maxSlugLen = 80

// Slug turns an arbitrary string into a filesystem-safe lowercase slug.
// Rules:
//   - Lowercase ASCII letters, digits, and dashes only.
//   - Any other rune (spaces, punctuation, accents, slashes, dots) becomes "-".
//   - Repeated dashes collapse to one.
//   - Leading/trailing dashes are trimmed.
//   - Truncated to maxSlugLen, trimming again.
//   - Empty input returns "".
//
// The function is pure and deterministic. It never returns absolute paths,
// path separators, or "..".
func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			// Non-ASCII letter/digit (e.g. á, ñ). Treat as separator to keep
			// filenames portable. We do not attempt transliteration.
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > maxSlugLen {
		out = strings.TrimRight(out[:maxSlugLen], "-")
	}
	return out
}

// observationFilename returns the stable filename for a single observation,
// including the ".md" suffix. Collisions are resolved by appending a stable
// suffix derived from the observation ID.
//
// taken is the set of filenames already assigned in this batch. The caller
// must update it after each successful assignment.
func observationFilename(o engram.Observation, taken map[string]bool) string {
	base := Slug(o.Title)
	if base == "" {
		base = "observation-" + Slug(o.ID)
		if base == "observation-" {
			base = "observation"
		}
	}

	name := base + ".md"
	if !taken[name] {
		return name
	}
	// Collision: append the slugged ID for stable disambiguation.
	idSlug := Slug(o.ID)
	if idSlug == "" {
		idSlug = "x"
	}
	return base + "-" + idSlug + ".md"
}

const observationTemplate = `---
type: observation
generated_by: silo
source: engram
observation_id: {{ .ID }}
observation_type: {{ .Type }}
project: {{ .Project }}
---

# {{ .DisplayTitle }}

{{ .Content }}

## Metadata

- ID: {{ .ID }}
- Type: {{ .Type }}
- Project: {{ .Project }}
- Created: {{ .Created }}
`

type observationView struct {
	ID           string
	Type         string
	Project      string
	DisplayTitle string
	Content      string
	Created      string
}

func newObservationView(o engram.Observation) observationView {
	title := strings.TrimSpace(o.Title)
	if title == "" {
		title = "observation " + o.ID
	}
	created := ""
	if !o.CreatedAt.IsZero() {
		// CreatedAt comes from the source data, not from "now". Safe to write
		// without breaking idempotency.
		created = o.CreatedAt.UTC().Format(time.RFC3339)
	}
	return observationView{
		ID:           o.ID,
		Type:         o.Type,
		Project:      o.Project,
		DisplayTitle: title,
		Content:      strings.TrimSpace(o.Content),
		Created:      created,
	}
}

// ObservationFilenames returns a stable map of {observation ID -> filename}
// using the exact same collision-resolution rules RenderObservations uses
// when writing the Raw/Observations folder. Exposed so other layers (e.g.
// the Curated renderer) can build Obsidian links that always resolve to
// the right Raw note.
func ObservationFilenames(obs []engram.Observation) map[string]string {
	if len(obs) == 0 {
		return map[string]string{}
	}
	sorted := make([]engram.Observation, len(obs))
	copy(sorted, obs)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	out := make(map[string]string, len(sorted))
	taken := make(map[string]bool, len(sorted))
	for _, o := range sorted {
		name := observationFilename(o, taken)
		taken[name] = true
		out[o.ID] = name
	}
	return out
}

// RenderObservations returns a map of {filename -> markdown content} for the
// given observations. Rendering is deterministic and idempotent: the same
// input slice produces the same output map.
//
// Empty input returns a one-entry map containing a README.md placeholder.
func RenderObservations(obs []engram.Observation) (map[string]string, error) {
	if len(obs) == 0 {
		return map[string]string{
			"README.md": emptyObservationsReadme,
		}, nil
	}

	// Sort by ID for stable collision resolution across runs. Order matters
	// because the first occurrence of a slug "wins" the bare name.
	sorted := make([]engram.Observation, len(obs))
	copy(sorted, obs)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	tmpl, err := template.New("observation").Parse(observationTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse observation template: %w", err)
	}

	out := make(map[string]string, len(sorted))
	taken := make(map[string]bool, len(sorted))

	for _, o := range sorted {
		name := observationFilename(o, taken)
		if taken[name] {
			// Should never happen: ID-suffixed name collided too. Bail loudly.
			return nil, fmt.Errorf("filename collision for observation %q at %q", o.ID, name)
		}
		taken[name] = true

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, newObservationView(o)); err != nil {
			return nil, fmt.Errorf("render observation %q: %w", o.ID, err)
		}
		out[name] = buf.String()
	}

	if len(out) == 0 {
		return nil, errors.New("internal: no observations rendered despite non-empty input")
	}
	return out, nil
}

const emptyObservationsReadme = `---
type: observations-index
generated_by: silo
source: engram
---

# Observations

No observations were found for this project.

When ` + "`silo sync`" + ` finds observations in Engram, each one is written here as an individual Markdown note.
`
