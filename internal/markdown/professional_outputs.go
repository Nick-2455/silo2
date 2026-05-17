package markdown

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/nicolasperalta/silo2/internal/identity"
)

// Professional outputs (CV, LinkedIn, Bio).
//
// These are timestamp-free, LLM-free, deterministic projections of a single
// identity.Identity value. Every helper here is pure: same Identity in,
// byte-identical Markdown out. That is what lets `silo outputs` be safely
// idempotent on top of WriteNoteIfAbsent.
//
// Templates intentionally include `TODO:` sections for things Silo cannot
// derive (Experience, Education, Certifications). The downstream
// `silo outputs` command writes these files via WriteNoteIfAbsent, so any
// human edits to those TODO sections survive forever; subsequent runs see
// the file exists and skip it. To regenerate a stale seed, the user
// deletes the file.

const cvTemplate = `---
type: cv
generated_by: silo
source: {{ .Source }}
---

# Curriculum Vitae

## Name
{{ .Identity.Name }}

## Role
{{ .Identity.Role }}

## Skills
{{- if .Identity.Skills }}
{{- range .Identity.Skills }}
- {{ . }}
{{- end }}
{{- else }}
- (none yet)
{{- end }}

## Projects
{{- if .Identity.Projects }}
{{- range .Identity.Projects }}
- **{{ .Name }}** ({{ .Status }}) — {{ .Description }}
{{- end }}
{{- else }}
- (none yet)
{{- end }}

## Highlights
{{- if .Identity.Evidence }}
{{- range .Identity.Evidence }}
- {{ .Summary }} _(source: {{ .Source }})_
{{- end }}
{{- else }}
- (none yet)
{{- end }}

## Experience
TODO: list previous roles, dates, and main contributions.

## Education
TODO: list degrees, institutions, and dates.

## Certifications
TODO: list relevant certifications.
`

const linkedInTemplate = `---
type: linkedin
generated_by: silo
source: {{ .Source }}
---

# LinkedIn

## Headline
{{ .Headline }}

## About
{{ .AboutParagraph }}

## Featured Projects
{{- if .Identity.Projects }}
{{- range .Identity.Projects }}
- **{{ .Name }}**: {{ .Description }}
{{- end }}
{{- else }}
- (none yet)
{{- end }}

## Skills
{{- if .Identity.Skills }}
{{- range .Identity.Skills }}
- {{ . }}
{{- end }}
{{- else }}
- (none yet)
{{- end }}

## Experience
TODO: paste roles, companies, dates, achievements.

## Education
TODO: paste degrees and institutions.
`

const bioTemplate = `---
type: bio
generated_by: silo
source: {{ .Source }}
---

# Professional Bio

## Short Bio
{{ .ShortBio }}

## Medium Bio
{{ .MediumBio }}

## Long Bio
{{ .LongBio }}

## Key Themes
{{- if .Themes }}
{{- range .Themes }}
- {{ . }}
{{- end }}
{{- else }}
- (none yet)
{{- end }}
`

// outputsView is the template input. Source is a short label (e.g.
// "curated" or "raw/engram") embedded in the frontmatter so a reader of
// a generated file knows where its data came from. It is NOT used in any
// rendering logic, only echoed.
type outputsView struct {
	Identity *identity.Identity
	Source   string

	// Pre-computed strings to keep templates dumb (no conditionals).
	Headline       string
	AboutParagraph string
	ShortBio       string
	MediumBio      string
	LongBio        string
	Themes         []string
}

// RenderProfessionalOutputs returns the {filename -> markdown content}
// map for CV.md, LinkedIn.md, and ProfessionalBio.md. The source label
// is embedded in each file's frontmatter for traceability.
//
// Deterministic on (ident, source): same input, same bytes.
func RenderProfessionalOutputs(ident *identity.Identity, source string) (map[string]string, error) {
	if ident == nil {
		return nil, errors.New("identity is nil")
	}
	src := strings.TrimSpace(source)
	if src == "" {
		src = "unknown"
	}

	view := outputsView{
		Identity:       ident,
		Source:         src,
		Headline:       buildHeadline(ident),
		AboutParagraph: buildAbout(ident),
		ShortBio:       buildShortBio(ident),
		MediumBio:      buildMediumBio(ident),
		LongBio:        buildLongBio(ident),
		Themes:         buildThemes(ident),
	}

	out := make(map[string]string, 3)
	for name, tmpl := range map[string]string{
		"CV.md":              cvTemplate,
		"LinkedIn.md":        linkedInTemplate,
		"ProfessionalBio.md": bioTemplate,
	} {
		s, err := renderOutputOne(tmpl, view)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", name, err)
		}
		out[name] = s
	}
	return out, nil
}

func renderOutputOne(tmpl string, view outputsView) (string, error) {
	t, err := template.New("out").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, view); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// --- Deterministic prose builders -----------------------------------------
//
// These never call time.Now, never randomize, never call external services.
// Everything is a string-join of stable Identity fields. That is the whole
// reason these outputs are safe to write as one-shot seeds.

func buildHeadline(i *identity.Identity) string {
	role := strings.TrimSpace(i.Role)
	areas := strings.TrimSpace(joinHuman(i.Areas))
	switch {
	case role != "" && areas != "":
		return role + " — " + areas
	case role != "":
		return role
	case areas != "":
		return areas
	default:
		return i.Name
	}
}

func buildAbout(i *identity.Identity) string {
	parts := []string{}
	if i.Name != "" {
		intro := i.Name
		if i.Role != "" {
			intro += " is a " + i.Role + "."
		} else {
			intro += "."
		}
		parts = append(parts, intro)
	}
	if len(i.Areas) > 0 {
		parts = append(parts, "Focused on "+joinHuman(i.Areas)+".")
	}
	if len(i.Skills) > 0 {
		parts = append(parts, "Core skills: "+joinHuman(i.Skills)+".")
	}
	if len(i.Projects) > 0 {
		names := make([]string, 0, len(i.Projects))
		for _, p := range i.Projects {
			names = append(names, p.Name)
		}
		parts = append(parts, "Active around "+joinHuman(names)+".")
	}
	if len(parts) == 0 {
		return "TODO: write About section."
	}
	return strings.Join(parts, " ")
}

func buildShortBio(i *identity.Identity) string {
	if i.Name == "" && i.Role == "" {
		return "TODO: write a short bio."
	}
	name := i.Name
	if name == "" {
		name = "This person"
	}
	role := i.Role
	if role == "" {
		role = "professional"
	}
	if len(i.Areas) > 0 {
		return name + ", " + role + ", focused on " + joinHuman(i.Areas) + "."
	}
	return name + ", " + role + "."
}

func buildMediumBio(i *identity.Identity) string {
	parts := []string{buildShortBio(i)}
	if len(i.Skills) > 0 {
		parts = append(parts, "Works primarily with "+joinHuman(i.Skills)+".")
	}
	if len(i.Projects) > 0 {
		names := make([]string, 0, len(i.Projects))
		for _, p := range i.Projects {
			names = append(names, p.Name)
		}
		parts = append(parts, "Current projects include "+joinHuman(names)+".")
	}
	return strings.Join(parts, " ")
}

func buildLongBio(i *identity.Identity) string {
	parts := []string{buildMediumBio(i)}
	if len(i.Goals) > 0 {
		parts = append(parts, "Driving goals: "+joinHuman(i.Goals)+".")
	}
	if len(i.Interests) > 0 {
		parts = append(parts, "Outside of core work, interested in "+joinHuman(i.Interests)+".")
	}
	if len(i.Evidence) > 0 {
		// Pull up to 3 evidence summaries to anchor the bio in real signals.
		n := len(i.Evidence)
		if n > 3 {
			n = 3
		}
		summaries := make([]string, 0, n)
		for _, e := range i.Evidence[:n] {
			s := strings.TrimSpace(e.Summary)
			if s != "" {
				summaries = append(summaries, s)
			}
		}
		if len(summaries) > 0 {
			parts = append(parts, "Recent threads: "+joinHuman(summaries)+".")
		}
	}
	return strings.Join(parts, " ")
}

func buildThemes(i *identity.Identity) []string {
	// Themes = Areas ∪ Interests, deduped, stable order (Areas first,
	// then Interests; both already sorted by BuildIdentity).
	seen := make(map[string]bool, len(i.Areas)+len(i.Interests))
	themes := make([]string, 0, len(i.Areas)+len(i.Interests))
	for _, a := range append(append([]string(nil), i.Areas...), i.Interests...) {
		if a == "" || seen[a] {
			continue
		}
		seen[a] = true
		themes = append(themes, a)
	}
	return themes
}

// joinHuman joins items with commas and an Oxford-comma "and" before the
// last one. Pure string work; no localization, English on purpose because
// the artifacts themselves are English (Persona Scope rule).
func joinHuman(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}
