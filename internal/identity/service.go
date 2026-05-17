package identity

import (
	"sort"
	"strings"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/engram"
)

func BuildIdentity(obs []engram.Observation, cfg *config.Config) (*Identity, error) {
	ident := &Identity{
		Name:    "Nicolas Peralta",
		Role:    "Software Architect",
		Outputs: DefaultOutputs(),
	}
	if cfg != nil && strings.TrimSpace(cfg.IdentityName) != "" {
		ident.Name = strings.TrimSpace(cfg.IdentityName)
	}

	// MVP heuristics: scan observation titles/contents for simple signals.
	skills := map[string]bool{}
	areas := map[string]bool{}
	interests := map[string]bool{}
	projects := map[string]Project{}

	for _, o := range obs {
		// Distinguish curated synthetic observations from real Engram rows
		// in the Evidence list. The "curated:" prefix is set by
		// internal/curated.LoadCurated; we keep the check on the prefix
		// (not an import) to avoid an import cycle and to keep this file
		// generic — anything that respects the prefix contract works.
		source := "Engram " + o.ID
		if strings.HasPrefix(o.ID, "curated:") {
			source = "Curated " + strings.TrimPrefix(o.ID, "curated:")
		}
		ident.Evidence = append(ident.Evidence, Evidence{
			Source:  source,
			Summary: o.Title,
		})

		t := strings.ToLower(o.Title + "\n" + o.Content)

		if strings.Contains(t, "go") {
			skills["Go"] = true
		}
		if strings.Contains(t, "swiftui") {
			skills["SwiftUI"] = true
		}
		if strings.Contains(t, "architecture") || strings.Contains(t, "architect") {
			skills["Architecture"] = true
		}
		if strings.Contains(t, "developer tooling") {
			areas["Developer Tooling"] = true
		}
		if strings.Contains(t, "knowledge") {
			areas["Knowledge Management"] = true
		}
		if strings.Contains(t, "local") || strings.Contains(t, "local-first") {
			interests["Local-first software"] = true
		}
		if strings.Contains(t, "obsidian") {
			projects["Obsidian"] = Project{Name: "Obsidian", Description: "Knowledge workspace used as the human interface.", Status: "external"}
		}
		if strings.Contains(t, "engram") {
			projects["Engram"] = Project{Name: "Engram", Description: "Persistent memory store and source of truth.", Status: "active"}
		}
		if strings.Contains(t, "silo") {
			projects["Silo"] = Project{Name: "Silo", Description: "Bridge that projects Engram knowledge into Markdown for Obsidian.", Status: "active"}
		}
	}

	// Stable defaults if mock data is replaced with something sparse.
	if len(projects) == 0 {
		projects["Silo"] = Project{Name: "Silo", Description: "Bridge that projects Engram knowledge into Markdown for Obsidian.", Status: "active"}
	}

	ident.Skills = keysSorted(skills)
	ident.Areas = keysSorted(areas)
	ident.Interests = keysSorted(interests)
	ident.Projects = projectsSorted(projects)

	// Minimal goals for the MVP narrative.
	ident.Goals = []string{
		"Keep Engram as the source of truth",
		"Generate readable, editable Markdown notes for Obsidian",
		"Maintain a minimal, dependency-free Go codebase",
	}

	return ident, nil
}

func keysSorted(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func projectsSorted(m map[string]Project) []Project {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	out := make([]Project, 0, len(names))
	for _, n := range names {
		out = append(out, m[n])
	}
	return out
}
