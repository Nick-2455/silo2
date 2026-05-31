package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func siloRecommendTool() mcp.Tool {
	return mcp.NewTool("silo_recommend",
		mcp.WithDescription("Recommend what to do next based on profile, seeds, and free time. Returns markdown with ranked suggestions."),
		mcp.WithString("date", mcp.Description("Date in YYYY-MM-DD format. Defaults to today.")),
	)
}

func handleSiloRecommend(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	vaultPath := deps.Config.VaultPath
	if vaultPath == "" {
		vaultPath = "./vault"
	}

	// Read profile for context.
	profilePath := filepath.Join(vaultPath, profileNotePath)
	profile := ProfileData{}
	if data, err := os.ReadFile(profilePath); err == nil {
		profile = parseProfileFrontmatter(string(data))
	}

	// Scan Inbox/open/ seeds.
	inboxDir := filepath.Join(vaultPath, "Inbox", "open")
	seeds := scanOpenSeeds(inboxDir)

	// Build markdown recommendation.
	md := renderRecommendMarkdown(profile, seeds)
	return jsonResult(map[string]any{"markdown": md, "seeds_found": len(seeds)})
}

// seedSummary holds minimal info extracted from an open seed file.
type seedSummary struct {
	Title string
	Path  string
	Tags  []string
}

func scanOpenSeeds(dir string) []seedSummary {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var seeds []seedSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		if strings.EqualFold(e.Name(), "README.md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		s := seedSummary{
			Title: parseSeedTitle(string(data)),
			Path:  filepath.Join("Inbox", "open", e.Name()),
		}
		seeds = append(seeds, s)
	}
	return seeds
}

func parseSeedTitle(raw string) string {
	// Use the first H1 heading as title, otherwise the filename.
	lines := strings.Split(raw, "\n")
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(trimmed[2:])
		}
	}
	return "(untitled)"
}

func renderRecommendMarkdown(profile ProfileData, seeds []seedSummary) string {
	var b strings.Builder
	b.WriteString("## Recomendaciones\n\n")

	if len(profile.CurrentFocus) > 0 {
		b.WriteString("**Foco actual:** ")
		b.WriteString(strings.Join(profile.CurrentFocus, ", "))
		b.WriteString("\n\n")
	}

	if len(seeds) == 0 {
		b.WriteString("*No hay seeds abiertos en Inbox/open. Guardá algo nuevo con `silo save`.*\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("### Ver ahora (%d seeds)\n\n", len(seeds)))
	for i, s := range seeds {
		if i >= 5 {
			b.WriteString(fmt.Sprintf("\n*...y %d seeds más en Inbox/open.*\n", len(seeds)-5))
			break
		}
		b.WriteString(fmt.Sprintf("- **%s** — `%s`\n", s.Title, s.Path))
	}

	b.WriteString("\n---\n")
	b.WriteString("*v1: basado en seeds abiertos. El motor de recomendación completo (Phase 2)\n")
	b.WriteString("usará tu perfil, horario y prioridades para rankear sugerencias.*\n")
	return b.String()
}
