package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/recommend"
)

func TestSiloRecommendTool_RequiresFreeSlots(t *testing.T) {
	tool := siloRecommendTool()
	if !containsString(tool.InputSchema.Required, "free_slots") {
		t.Fatalf("required args = %v, want free_slots", tool.InputSchema.Required)
	}
	if _, ok := tool.InputSchema.Properties["free_slots"]; !ok {
		t.Fatalf("properties = %v, want free_slots", tool.InputSchema.Properties)
	}
	if _, ok := tool.InputSchema.Properties["date"]; ok {
		t.Fatalf("date should not remain in tool schema: %v", tool.InputSchema.Properties)
	}
}

func TestHandleSiloRecommend_ValidationAndResponse(t *testing.T) {
	withRecommendDeps(t, t.TempDir(), nil)
	writeRecommendProfileFile(t, deps.Config.VaultPath)
	writeRecommendSeedFile(t, deps.Config.VaultPath, "seed-001.md", "Go testing", map[string]string{"type": "article", "tags": "go,testing", "estimated_minutes": "25"})

	tests := []struct {
		name         string
		args         map[string]any
		wantErr      string
		wantMinutes  float64
		wantSeedScan float64
	}{
		{name: "missing free slots", args: map[string]any{}, wantErr: "free_slots is required"},
		{name: "empty free slots", args: map[string]any{"free_slots": []any{}}, wantErr: "no free time available"},
		{name: "missing start", args: map[string]any{"free_slots": []any{map[string]any{"end": "2026-06-17T15:00:00Z"}}}, wantErr: "free_slots[0]: invalid start: missing RFC3339 timestamp"},
		{name: "missing end", args: map[string]any{"free_slots": []any{map[string]any{"start": "2026-06-17T14:00:00Z"}}}, wantErr: "free_slots[0]: invalid end: missing RFC3339 timestamp"},
		{name: "invalid start", args: map[string]any{"free_slots": []any{map[string]any{"start": "nope", "end": "2026-06-17T15:00:00Z"}}}, wantErr: "free_slots[0]: invalid start:"},
		{name: "invalid end", args: map[string]any{"free_slots": []any{map[string]any{"start": "2026-06-17T14:00:00Z", "end": "nope"}}}, wantErr: "free_slots[0]: invalid end:"},
		{name: "end before start", args: map[string]any{"free_slots": []any{map[string]any{"start": "2026-06-17T15:00:00Z", "end": "2026-06-17T15:00:00Z"}}}, wantErr: "free_slots[0]: end must be after start"},
		{name: "valid slots", args: map[string]any{"free_slots": []any{map[string]any{"start": "2026-06-17T14:00:00Z", "end": "2026-06-17T15:30:00Z"}}}, wantMinutes: 90, wantSeedScan: 1},
		{name: "normalizes non utc slots", args: map[string]any{"free_slots": []any{map[string]any{"start": "2026-06-17T10:00:00-04:00", "end": "2026-06-17T11:30:00-04:00"}}}, wantMinutes: 90, wantSeedScan: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := handleSiloRecommend(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: tt.args}})
			if err != nil {
				t.Fatalf("handleSiloRecommend() error = %v", err)
			}

			if tt.wantErr != "" {
				if !result.IsError {
					t.Fatalf("result.IsError = false, want true")
				}
				if !strings.Contains(toolResultText(t, result), tt.wantErr) {
					t.Fatalf("tool error = %q, want substring %q", toolResultText(t, result), tt.wantErr)
				}
				return
			}

			if result.IsError {
				t.Fatalf("result.IsError = true, want false: %s", toolResultText(t, result))
			}

			var payload map[string]any
			if err := json.Unmarshal([]byte(toolResultText(t, result)), &payload); err != nil {
				t.Fatalf("Unmarshal(result) error = %v", err)
			}
			if payload["free_minutes"] != tt.wantMinutes {
				t.Fatalf("free_minutes = %v, want %v", payload["free_minutes"], tt.wantMinutes)
			}
			if payload["seeds_considered"] != tt.wantSeedScan {
				t.Fatalf("seeds_considered = %v, want %v", payload["seeds_considered"], tt.wantSeedScan)
			}
			if _, ok := payload["free_slots"]; ok {
				t.Fatalf("payload unexpectedly includes free_slots: %v", payload)
			}
			if _, ok := payload["recommendations"].([]any); !ok {
				t.Fatalf("recommendations payload missing array: %v", payload)
			}
		})
	}
}

func TestParseFreeSlots_NormalizesToUTC(t *testing.T) {
	t.Parallel()

	slots, total, err := parseFreeSlots(mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{
		"free_slots": []any{map[string]any{"start": "2026-06-17T10:00:00-04:00", "end": "2026-06-17T11:30:00-04:00"}},
	}}})
	if err != nil {
		t.Fatalf("parseFreeSlots() error = %v", err)
	}
	if total != 90 {
		t.Fatalf("total = %d, want 90", total)
	}
	if !reflect.DeepEqual(slots, []FreeSlot{{Start: "2026-06-17T14:00:00Z", End: "2026-06-17T15:30:00Z"}}) {
		t.Fatalf("slots = %#v", slots)
	}
}

func TestHandleSiloRecommend_PassesProductiveHoursToEngine(t *testing.T) {
	vaultDir := t.TempDir()
	stub := &recordingEngine{result: []recommend.Recommendation{{Title: "Focused work", Score: 42}}}
	withRecommendDeps(t, vaultDir, [][2]string{{"08:00", "12:00"}})
	writeRecommendProfileFile(t, vaultDir)
	writeRecommendSeedFile(t, vaultDir, "seed-001.md", "Focused work", map[string]string{"type": "article", "tags": "go", "estimated_minutes": "30"})

	oldFactory := newRecommendEngine
	newRecommendEngine = func() recommend.Engine { return stub }
	t.Cleanup(func() { newRecommendEngine = oldFactory })

	result, err := handleSiloRecommend(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{
		"free_slots": []any{map[string]any{"start": "2026-06-17T14:00:00Z", "end": "2026-06-17T15:00:00Z"}},
	}}})
	if err != nil {
		t.Fatalf("handleSiloRecommend() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("result.IsError = true, want false: %s", toolResultText(t, result))
	}
	if !reflect.DeepEqual(stub.hints.ProductiveHours, [][2]string{{"08:00", "12:00"}}) {
		t.Fatalf("hints.ProductiveHours = %v", stub.hints.ProductiveHours)
	}
	if stub.freeMinutes != 60 {
		t.Fatalf("freeMinutes = %d, want 60", stub.freeMinutes)
	}
	if len(stub.seeds) != 1 {
		t.Fatalf("seeds length = %d, want 1", len(stub.seeds))
	}
}

func TestSiloRecommendDeterminism(t *testing.T) {
	vaultDir := t.TempDir()
	withRecommendDeps(t, vaultDir, nil)
	writeRecommendProfileFile(t, vaultDir)
	writeRecommendSeedFile(t, vaultDir, "seed-001.md", "Go testing", map[string]string{"type": "article", "tags": "go,testing", "estimated_minutes": "25"})
	writeRecommendSeedFile(t, vaultDir, "seed-002.md", "Gardening", map[string]string{"type": "video", "tags": "garden", "estimated_minutes": "45"})

	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{
		"free_slots": []any{
			map[string]any{"start": "2026-06-17T14:00:00Z", "end": "2026-06-17T15:00:00Z"},
			map[string]any{"start": "2026-06-17T16:00:00Z", "end": "2026-06-17T16:30:00Z"},
		},
	}}}

	first, err := handleSiloRecommend(context.Background(), req)
	if err != nil {
		t.Fatalf("first handleSiloRecommend() error = %v", err)
	}
	second, err := handleSiloRecommend(context.Background(), req)
	if err != nil {
		t.Fatalf("second handleSiloRecommend() error = %v", err)
	}

	if toolResultText(t, first) != toolResultText(t, second) {
		t.Fatalf("first result = %q, second result = %q", toolResultText(t, first), toolResultText(t, second))
	}
}

type recordingEngine struct {
	profile     recommend.Profile
	seeds       []recommend.SeedInput
	freeMinutes int
	hints       recommend.Hints
	result      []recommend.Recommendation
}

func (e *recordingEngine) Recommend(profile recommend.Profile, seeds []recommend.SeedInput, freeMinutes int) ([]recommend.Recommendation, error) {
	return e.result, nil
}

func (e *recordingEngine) RecommendWithHints(profile recommend.Profile, seeds []recommend.SeedInput, freeMinutes int, hints recommend.Hints) ([]recommend.Recommendation, error) {
	e.profile = profile
	e.seeds = append([]recommend.SeedInput(nil), seeds...)
	e.freeMinutes = freeMinutes
	e.hints = hints
	return e.result, nil
}

func withRecommendDeps(t *testing.T, vaultDir string, productiveHours [][2]string) {
	t.Helper()
	SetDeps(Deps{Config: &config.Config{VaultPath: vaultDir, ProductiveHours: productiveHours}})
	oldFactory := newRecommendEngine
	newRecommendEngine = func() recommend.Engine { return recommend.NewEngine() }
	t.Cleanup(func() { newRecommendEngine = oldFactory })
}

func writeRecommendProfileFile(t *testing.T, vaultDir string) {
	t.Helper()
	path := filepath.Join(vaultDir, profileNotePath)
	mustWriteFile(t, path, "---\n{\"current_focus\":[\"Go\"]}\n---\n")
}

func writeRecommendSeedFile(t *testing.T, vaultDir, name, title string, frontmatter map[string]string) {
	t.Helper()
	path := filepath.Join(vaultDir, "Inbox", "open", name)
	var b strings.Builder
	b.WriteString("---\n")
	for _, key := range []string{"type", "tags", "estimated_minutes"} {
		if value, ok := frontmatter[key]; ok {
			b.WriteString(key)
			b.WriteString(": ")
			b.WriteString(value)
			b.WriteString("\n")
		}
	}
	b.WriteString("---\n\n# ")
	b.WriteString(title)
	b.WriteString("\n")
	mustWriteFile(t, path, b.String())
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func toolResultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("tool result has no content")
	}
	text, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("result content = %#v, want text", result.Content[0])
	}
	return text.Text
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
