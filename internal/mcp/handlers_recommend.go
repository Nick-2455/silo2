package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/nicolasperalta/silo2/internal/recommend"
)

var newRecommendEngine = func() recommend.Engine { return recommend.NewEngine() }

type FreeSlot struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

func siloRecommendTool() mcp.Tool {
	return mcp.NewTool("silo_recommend",
		mcp.WithDescription("Recommend what to do next based on profile, seeds, and free time. Returns deterministic JSON suggestions."),
		mcp.WithArray("free_slots",
			mcp.Required(),
			mcp.Description("Free time slots as RFC3339 start/end pairs."),
			mcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"start": map[string]any{"type": "string", "description": "RFC3339 start timestamp"},
					"end":   map[string]any{"type": "string", "description": "RFC3339 end timestamp"},
				},
				"required": []string{"start", "end"},
			}),
		),
	)
}

func handleSiloRecommend(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	_ = ctx

	vaultPath := "./vault"
	if deps.Config != nil && deps.Config.VaultPath != "" {
		vaultPath = deps.Config.VaultPath
	}

	_, freeMinutes, err := parseFreeSlots(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	profilePath := filepath.Join(vaultPath, profileNotePath)
	profile := recommend.Profile{}
	if data, err := os.ReadFile(profilePath); err == nil {
		profile = recommendProfile(parseProfileFrontmatter(string(data)))
	}

	inboxDir := filepath.Join(vaultPath, "Inbox", "open")
	seeds := loadRecommendSeedInputs(inboxDir)

	hints := recommend.Hints{}
	if deps.Config != nil {
		hints.ProductiveHours = deps.Config.ProductiveHours
	}

	recs, err := newRecommendEngine().RecommendWithHints(profile, seeds, freeMinutes, hints)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return jsonResult(map[string]any{
		"recommendations":  recs,
		"free_minutes":     freeMinutes,
		"seeds_considered": len(seeds),
	})
}

func parseFreeSlots(req mcp.CallToolRequest) ([]FreeSlot, int, error) {
	args := req.GetArguments()
	raw, ok := args["free_slots"]
	if !ok || raw == nil {
		return nil, 0, fmt.Errorf("free_slots is required")
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, 0, fmt.Errorf("free_slots is required")
	}
	if len(items) == 0 {
		return nil, 0, fmt.Errorf("no free time available")
	}

	slots := make([]FreeSlot, 0, len(items))
	total := 0
	for i, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, 0, fmt.Errorf("free_slots[%d]: invalid start: missing RFC3339 timestamp", i)
		}
		startRaw, ok := entry["start"].(string)
		if !ok || strings.TrimSpace(startRaw) == "" {
			return nil, 0, fmt.Errorf("free_slots[%d]: invalid start: missing RFC3339 timestamp", i)
		}
		endRaw, ok := entry["end"].(string)
		if !ok || strings.TrimSpace(endRaw) == "" {
			return nil, 0, fmt.Errorf("free_slots[%d]: invalid end: missing RFC3339 timestamp", i)
		}

		start, err := time.Parse(time.RFC3339, startRaw)
		if err != nil {
			return nil, 0, fmt.Errorf("free_slots[%d]: invalid start: %v", i, err)
		}
		end, err := time.Parse(time.RFC3339, endRaw)
		if err != nil {
			return nil, 0, fmt.Errorf("free_slots[%d]: invalid end: %v", i, err)
		}
		if !end.After(start) {
			return nil, 0, fmt.Errorf("free_slots[%d]: end must be after start", i)
		}

		startUTC := start.UTC()
		endUTC := end.UTC()

		slots = append(slots, FreeSlot{Start: startUTC.Format(time.RFC3339), End: endUTC.Format(time.RFC3339)})
		total += int(end.Sub(start).Minutes())
	}

	return slots, total, nil
}

func recommendProfile(profile ProfileData) recommend.Profile {
	return recommend.Profile{
		CurrentFocus:  profile.CurrentFocus,
		Interests:     profile.Interests,
		LearningGoals: profile.LearningGoals,
	}
}

func loadRecommendSeedInputs(dir string) []recommend.SeedInput {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	seeds := make([]recommend.SeedInput, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		if strings.EqualFold(entry.Name(), "README.md") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		frontmatter := parseSeedFrontmatter(string(data))
		seeds = append(seeds, recommend.SeedInput{
			Title:         extractSeedHeading(string(data)),
			Path:          filepath.Join("Inbox", "open", entry.Name()),
			Frontmatter:   frontmatter,
			EstimatedMins: parseEstimatedMinutes(frontmatter["estimated_minutes"]),
			Tags:          parseSeedTags(frontmatter["tags"]),
		})
	}

	return seeds
}

func parseSeedFrontmatter(raw string) map[string]string {
	fm := extractFrontmatter(raw)
	if fm == "" {
		return nil
	}

	result := map[string]string{}
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result
}

func extractSeedHeading(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	return "(untitled)"
}

func parseSeedTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	tags := make([]string, 0, len(parts))
	for _, part := range parts {
		if tag := strings.TrimSpace(part); tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func parseEstimatedMinutes(raw string) int {
	minutes, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return minutes
}
