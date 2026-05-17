package markdown

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"

	"github.com/nicolasperalta/silo2/internal/engram"
)

// Curated layer — human-editable notes seeded from observations.
//
// Contract enforced by this file:
//
//   - Output is a map of {relative path under Curated/ -> markdown content}.
//     Example key: "Architecture/silo-design.md".
//   - Rendering is deterministic: same input slice → same output map.
//   - Each curated note lists the Raw observations it was seeded from.
//   - This package only RENDERS. It does NOT decide whether to write. The
//     "do not overwrite human edits" rule lives in obsidian.WriteNoteIfAbsent
//     so the policy is enforced at the single filesystem boundary.
//
// Grouping rules (deterministic, no LLM, no SQLite):
//
//  1. Bucket selection (which Curated/<dir> a note lands in):
//     - If observation.TopicKey starts with a known prefix like
//       "architecture/", "project[s]/", "identity/", "career/" or
//       "learning/", that prefix decides the bucket. This matches how
//       Engram already namespaces topic_keys in practice.
//     - Otherwise, observation.Type maps to a bucket (architecture/design/
//       decision/pattern → Architecture; project/proposal/explore/
//       session* → Projects; identity → Identity; learning/career/skill →
//       Career).
//     - Unrecognized → Architecture (default catch-all, documented).
//
//  2. Note identity within a bucket:
//     - Observations sharing the same non-empty TopicKey collapse into a
//       single curated note. Note slug = slug(<topic tail>).
//     - Observations without TopicKey produce one note per observation.
//       Note slug = slug(title) (same rule as Raw).
//
// Trade-off documented on purpose: when new observations land later, the
// "Related Observations" list of an existing curated note may go stale,
// because we never overwrite the file. The user explicitly chose this
// safety over freshness. To refresh, the human deletes the curated note.
//
// All four bucket directories are guaranteed to appear at least as a
// README.md placeholder so the vault structure is predictable even when
// a project has zero observations for a bucket.

// CuratedBuckets is the canonical, ordered list of Curated subdirectories
// Silo manages. Order is the rendering order, also used by tests.
var CuratedBuckets = []string{"Architecture", "Projects", "Identity", "Career"}

const curatedTemplate = `---
type: curated
generated_by: silo
source: engram
topic_key: {{ .TopicKey }}
---

# {{ .Title }}

## Summary

TODO: Write human-curated summary.

## Related Observations

{{ range .Links }}- [[Raw/Observations/{{ . }}]]
{{ end }}
## Notes

TODO.
`

const curatedBucketReadme = `---
type: curated-index
generated_by: silo
source: engram
---

# %s

This folder holds human-curated notes for the **%s** bucket.

Silo seeds files here from Engram observations using ` + "`silo curate`" + `,
but never overwrites them once you start editing. Treat any file here as
yours.
`

type curatedView struct {
	Title    string
	TopicKey string // empty when grouping by single-observation
	Links    []string
}

// RenderCurated returns a map of {relative-path -> markdown content} for
// curated seed notes. Paths always use forward slashes and never include
// the leading "Curated/" prefix (the caller mounts them under that root).
//
// Empty input still returns one README.md per bucket so Curated/<bucket>/
// is discoverable in Obsidian. README files are NOT considered curated
// notes for overwrite-protection purposes (the caller passes them through
// the same WriteNoteIfAbsent, so once present they too are left alone).
func RenderCurated(obs []engram.Observation) (map[string]string, error) {
	out := make(map[string]string)

	// 1. Always emit bucket READMEs. Idempotent and bucket-stable.
	for _, b := range CuratedBuckets {
		out[b+"/README.md"] = fmt.Sprintf(curatedBucketReadme, b, b)
	}

	if len(obs) == 0 {
		return out, nil
	}

	// 2. Build stable Raw filename lookup so links resolve.
	rawNames := ObservationFilenames(obs)

	// 3. Group observations into curated notes.
	type group struct {
		bucket   string
		slug     string
		title    string
		topicKey string
		obsIDs   []string
	}
	groups := make(map[string]*group) // key = bucket+"/"+slug

	// Sort by ID up front so iteration order is deterministic.
	sorted := make([]engram.Observation, len(obs))
	copy(sorted, obs)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	for _, o := range sorted {
		bucket, slug, title := classify(o)
		if slug == "" {
			// Defensive: skip observations we cannot name. Should not happen
			// because classify() falls back to the ID slug.
			continue
		}
		key := bucket + "/" + slug
		g, ok := groups[key]
		if !ok {
			g = &group{
				bucket:   bucket,
				slug:     slug,
				title:    title,
				topicKey: strings.TrimSpace(o.TopicKey),
			}
			groups[key] = g
		}
		g.obsIDs = append(g.obsIDs, o.ID)
	}

	// 4. Render each group.
	tmpl, err := template.New("curated").Parse(curatedTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse curated template: %w", err)
	}

	// Iterate groups in deterministic order (path key).
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		g := groups[k]
		// Links derived from rawNames in obsID order, stable.
		links := make([]string, 0, len(g.obsIDs))
		seen := make(map[string]bool, len(g.obsIDs))
		idsSorted := append([]string(nil), g.obsIDs...)
		sort.Strings(idsSorted)
		for _, id := range idsSorted {
			name := rawNames[id]
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			// Strip .md for Obsidian wikilink style.
			links = append(links, strings.TrimSuffix(name, ".md"))
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, curatedView{
			Title:    g.title,
			TopicKey: g.topicKey,
			Links:    links,
		}); err != nil {
			return nil, fmt.Errorf("render curated %q: %w", k, err)
		}
		out[g.bucket+"/"+g.slug+".md"] = buf.String()
	}

	return out, nil
}

// classify returns (bucket, slug, displayTitle) for one observation.
//
// Bucket selection: topic_key prefix wins; otherwise type-based mapping;
// otherwise the catch-all "Architecture". Slug: topic_key tail when the
// observation has a topic_key (so siblings collapse into one note),
// otherwise the title (falling back to the ID).
func classify(o engram.Observation) (bucket, slug, title string) {
	tk := strings.ToLower(strings.TrimSpace(o.TopicKey))

	// Bucket from topic_key prefix.
	switch {
	case strings.HasPrefix(tk, "architecture/"), strings.HasPrefix(tk, "design/"), strings.HasPrefix(tk, "decision/"):
		bucket = "Architecture"
	case strings.HasPrefix(tk, "project/"), strings.HasPrefix(tk, "projects/"), strings.HasPrefix(tk, "proposal/"):
		bucket = "Projects"
	case strings.HasPrefix(tk, "identity/"), strings.HasPrefix(tk, "profile/"):
		bucket = "Identity"
	case strings.HasPrefix(tk, "career/"), strings.HasPrefix(tk, "learning/"), strings.HasPrefix(tk, "skill/"):
		bucket = "Career"
	default:
		bucket = bucketForType(o.Type)
	}

	// Slug + title.
	if tk != "" {
		if i := strings.IndexByte(tk, '/'); i >= 0 && i+1 < len(tk) {
			tail := tk[i+1:]
			slug = Slug(tail)
			title = humanize(tail)
		} else {
			slug = Slug(tk)
			title = humanize(tk)
		}
	} else {
		t := strings.TrimSpace(o.Title)
		if t == "" {
			slug = "observation-" + Slug(o.ID)
			title = "Observation " + o.ID
		} else {
			slug = Slug(t)
			if slug == "" {
				slug = "observation-" + Slug(o.ID)
			}
			title = t
		}
	}

	return bucket, slug, title
}

// bucketForType maps an Engram observation type to a Curated bucket. The
// mapping is intentionally small and additive; unknown types fall into
// Architecture so we never lose data, only group it conservatively.
func bucketForType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "architecture", "design", "decision", "pattern":
		return "Architecture"
	case "project", "proposal", "explore", "session_summary":
		return "Projects"
	case "identity":
		return "Identity"
	case "learning", "career", "skill":
		return "Career"
	default:
		return "Architecture"
	}
}

// humanize turns "engram-http-api" → "Engram Http Api" for a readable
// curated note title. Kept intentionally dumb; humans are expected to
// rewrite the heading anyway.
func humanize(s string) string {
	s = strings.NewReplacer("-", " ", "_", " ").Replace(s)
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	parts := strings.Fields(s)
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
