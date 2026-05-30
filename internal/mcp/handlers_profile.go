package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

// ProfileData is the JSON structure returned by get_profile_context and
// stored in Silo/Profile.md.
type ProfileData struct {
	Name            string   `json:"name,omitempty"`
	Role            string   `json:"role,omitempty"`
	Interests       []string `json:"interests,omitempty"`
	CurrentFocus    []string `json:"current_focus,omitempty"`
	LearningGoals   []string `json:"learning_goals,omitempty"`
	PreferredStyle  string   `json:"preferred_style,omitempty"`
	ActiveProjects  []string `json:"active_projects,omitempty"`
}

const profileNotePath = "Silo/Profile.md"

func getProfileContextTool() mcp.Tool {
	return mcp.NewTool("get_profile_context",
		mcp.WithDescription("Read or initialize Silo/Profile.md from the configured Obsidian vault and return its profile context as JSON."),
	)
}

func initProfileTool() mcp.Tool {
	return mcp.NewTool("init_profile",
		mcp.WithDescription("Create Silo/Profile.md in the configured Obsidian vault if missing. Uses Engram conservatively; creates an empty profile when no data exists."),
	)
}

func handleGetProfileContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return ensureProfileResult()
}

func handleInitProfile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return ensureProfileResult()
}

func ensureProfileResult() (*mcp.CallToolResult, error) {
	vaultPath := deps.Config.VaultPath
	if vaultPath == "" {
		vaultPath = "./vault"
	}

	targetPath := filepath.Join(vaultPath, profileNotePath)

	// Try to read existing profile.
	data, err := os.ReadFile(targetPath)
	if err != nil && !os.IsNotExist(err) {
		return mcp.NewToolResultError(fmt.Sprintf("profile: read %s: %v", targetPath, err)), nil
	}

	if err == nil {
		// File exists — parse frontmatter and return profile data.
		p := parseProfileFrontmatter(string(data))
		return jsonResult(map[string]any{"profile": p, "path": profileNotePath, "created": false})
	}

	// File does not exist — create a minimal one.
	profile := ProfileData{}
	content := renderProfileMarkdown(profile)

	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("profile: mkdir %s: %v", dir, err)), nil
	}
	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("profile: write %s: %v", targetPath, err)), nil
	}

	return jsonResult(map[string]any{"profile": profile, "path": profileNotePath, "created": true})
}

// parseProfileFrontmatter extracts ProfileData from YAML frontmatter in a
// Markdown profile file. Returns an empty profile on parse failure.
func parseProfileFrontmatter(raw string) ProfileData {
	// Simple YAML frontmatter extraction: find the first --- block.
	fm := extractFrontmatter(raw)
	if fm == "" {
		return ProfileData{}
	}

	var p ProfileData
	if err := json.Unmarshal([]byte(fm), &p); err != nil {
		// YAML frontmatter is close enough to JSON for our simple fields.
		// If JSON parsing fails, return empty profile.
		return ProfileData{}
	}
	return p
}

// extractFrontmatter returns the raw string between the first pair of "---" markers.
func extractFrontmatter(s string) string {
	if len(s) < 4 || s[:3] != "---" {
		return ""
	}
	rest := s[3:]
	// Skip the newline after opening ---.
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}
	end := -1
	for i := 0; i < len(rest)-2; i++ {
		if rest[i] == '\n' && rest[i+1] == '-' && rest[i+2] == '-' && rest[i+3] == '-' {
			end = i
			break
		}
	}
	if end < 0 {
		// No closing --- found; return the rest as-is.
		return rest
	}
	return rest[:end]
}

// renderProfileMarkdown generates a minimal Silo/Profile.md template.
func renderProfileMarkdown(p ProfileData) string {
	profileJSON, _ := json.MarshalIndent(p, "", "  ")
	return fmt.Sprintf(`---
%s
---

# Profile

Your profile is managed by Silo. Edit the frontmatter above to set your name,
role, interests, current focus, and learning goals.

## Active Projects

<!-- Add your active projects here -->
`, string(profileJSON))
}
