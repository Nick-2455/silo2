package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/engram"
)

func setupTestDeps(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	schedulePath := filepath.Join(dir, "schedule.json")
	cfg := &config.Config{
		VaultPath:    dir,
		SchedulePath: schedulePath,
	}
	SetDeps(Deps{
		Config: cfg,
		Engram: engram.NewMockClient(),
	})
}

func TestHandleAddScheduleEvent(t *testing.T) {
	setupTestDeps(t)

	args := map[string]any{
		"title":            "Test meeting",
		"start":            "10:00",
		"duration_minutes": float64(45),
		"type":             "fixed",
		"category":         "work",
		"days":             []any{"mon", "wed"},
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	result, err := handleAddScheduleEvent(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse result to verify.
	var parsed map[string]any
	if len(result.Content) > 0 {
		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			if jsonErr := json.Unmarshal([]byte(textContent.Text), &parsed); jsonErr != nil {
				t.Fatalf("failed to parse result: %v", jsonErr)
			}
		}
	}

	eventMap, ok := parsed["event"].(map[string]any)
	if !ok {
		t.Fatalf("expected event in result, got %#v", parsed)
	}
	if eventMap["title"] != "Test meeting" {
		t.Fatalf("expected 'Test meeting', got %v", eventMap["title"])
	}
	if eventMap["id"] == "" {
		t.Fatalf("expected generated id")
	}
}

func TestHandleAddScheduleEvent_InvalidType(t *testing.T) {
	setupTestDeps(t)

	args := map[string]any{
		"title":            "Bad type",
		"start":            "10:00",
		"duration_minutes": float64(30),
		"type":             "invalid",
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	result, err := handleAddScheduleEvent(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !isErrorResult(result) {
		t.Fatalf("expected error result, got success")
	}
}

func TestHandleAddScheduleEvent_InvalidStartTime(t *testing.T) {
	setupTestDeps(t)

	args := map[string]any{
		"title":            "Bad time",
		"start":            "25:00",
		"duration_minutes": float64(30),
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	result, err := handleAddScheduleEvent(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !isErrorResult(result) {
		t.Fatalf("expected error result, got success")
	}
}

func TestHandleListScheduleEvents_Empty(t *testing.T) {
	setupTestDeps(t)

	args := map[string]any{
		"date": "2026-06-01",
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	result, err := handleListScheduleEvents(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if len(result.Content) > 0 {
		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			json.Unmarshal([]byte(textContent.Text), &parsed)
		}
	}

	if count, ok := parsed["count"].(float64); !ok || count != 0 {
		t.Fatalf("expected count=0, got %v", parsed["count"])
	}
}

func TestHandleRemoveScheduleEvent(t *testing.T) {
	setupTestDeps(t)

	// First add an event.
	addArgs := map[string]any{
		"title":            "To be removed",
		"start":            "14:00",
		"duration_minutes": float64(60),
	}

	addReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: addArgs,
		},
	}

	addResult, err := handleAddScheduleEvent(context.Background(), addReq)
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	// Extract the event ID.
	var parsed map[string]any
	if len(addResult.Content) > 0 {
		if textContent, ok := mcp.AsTextContent(addResult.Content[0]); ok {
			json.Unmarshal([]byte(textContent.Text), &parsed)
		}
	}
	eventMap := parsed["event"].(map[string]any)
	eventID := eventMap["id"].(string)

	// Now remove it.
	removeArgs := map[string]any{"id": eventID}
	removeReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: removeArgs,
		},
	}

	removeResult, err := handleRemoveScheduleEvent(context.Background(), removeReq)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if isErrorResult(removeResult) {
		t.Fatalf("expected success, got error")
	}
}

func TestHandleRemoveScheduleEvent_NotFound(t *testing.T) {
	setupTestDeps(t)

	args := map[string]any{"id": "nonexistent"}
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	result, err := handleRemoveScheduleEvent(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isErrorResult(result) {
		t.Fatalf("expected error result for nonexistent ID")
	}
}

func TestHandleGetFreeSlots(t *testing.T) {
	setupTestDeps(t)

	// Add a busy event.
	addArgs := map[string]any{
		"title":            "Block",
		"start":            "10:00",
		"duration_minutes": float64(120),
		"days":             []any{"2026-06-01"},
	}
	addReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: addArgs,
		},
	}
	handleAddScheduleEvent(context.Background(), addReq)

	args := map[string]any{
		"date":  "2026-06-01",
		"start": "06:00",
		"end":   "22:00",
	}
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	result, err := handleGetFreeSlots(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if len(result.Content) > 0 {
		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			json.Unmarshal([]byte(textContent.Text), &parsed)
		}
	}

	slots, ok := parsed["slots"].([]any)
	if !ok {
		t.Fatalf("expected slots array, got %#v", parsed)
	}
	if len(slots) != 2 {
		t.Fatalf("expected 2 free slots (06:00-10:00, 12:00-22:00), got %d", len(slots))
	}
}

func TestHandlePreviewSchedule(t *testing.T) {
	setupTestDeps(t)

	args := map[string]any{
		"date":  "2026-06-01",
		"start": "06:00",
		"end":   "22:00",
	}
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}

	result, err := handlePreviewSchedule(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if len(result.Content) > 0 {
		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			json.Unmarshal([]byte(textContent.Text), &parsed)
		}
	}

	md, ok := parsed["markdown"].(string)
	if !ok || md == "" {
		t.Fatalf("expected non-empty markdown, got %v", parsed["markdown"])
	}
}

func TestHandleGetProfileContext(t *testing.T) {
	setupTestDeps(t)

	// Create a simple profile file.
	profilePath := filepath.Join(deps.Config.VaultPath, profileNotePath)
	profileDir := filepath.Dir(profilePath)
	os.MkdirAll(profileDir, 0o755)
	os.WriteFile(profilePath, []byte(`---
{"name": "Test User", "role": "Developer"}
---

# Profile
`), 0o644)

	result, err := handleGetProfileContext(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if len(result.Content) > 0 {
		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			json.Unmarshal([]byte(textContent.Text), &parsed)
		}
	}

	profileMap, ok := parsed["profile"].(map[string]any)
	if !ok {
		t.Fatalf("expected profile in result, got %#v", parsed)
	}
	if profileMap["name"] != "Test User" {
		t.Fatalf("expected 'Test User', got %v", profileMap["name"])
	}
}

func TestHandleGetProfileContext_CreatesIfMissing(t *testing.T) {
	setupTestDeps(t)

	result, err := handleGetProfileContext(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if len(result.Content) > 0 {
		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			json.Unmarshal([]byte(textContent.Text), &parsed)
		}
	}

	created, ok := parsed["created"].(bool)
	if !ok || !created {
		t.Fatalf("expected created=true, got %v", parsed["created"])
	}
}

func TestHandleSiloRecommend(t *testing.T) {
	setupTestDeps(t)

	// Create a profile.
	profilePath := filepath.Join(deps.Config.VaultPath, profileNotePath)
	profileDir := filepath.Dir(profilePath)
	os.MkdirAll(profileDir, 0o755)
	os.WriteFile(profilePath, []byte(`---
{"current_focus": ["Go", "Architecture"]}
---

# Profile
`), 0o644)

	// Create an open seed.
	inboxDir := filepath.Join(deps.Config.VaultPath, "Inbox", "open")
	os.MkdirAll(inboxDir, 0o755)
	os.WriteFile(filepath.Join(inboxDir, "seed-1.md"), []byte("# Learn Go Generics\n\nContent about generics.\n"), 0o644)

	result, err := handleSiloRecommend(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if len(result.Content) > 0 {
		if textContent, ok := mcp.AsTextContent(result.Content[0]); ok {
			json.Unmarshal([]byte(textContent.Text), &parsed)
		}
	}

	md, ok := parsed["markdown"].(string)
	if !ok || md == "" {
		t.Fatalf("expected non-empty markdown, got %v", parsed["markdown"])
	}
	seedsFound, ok := parsed["seeds_found"].(float64)
	if !ok || seedsFound != 1 {
		t.Fatalf("expected seeds_found=1, got %v", parsed["seeds_found"])
	}
}

func isErrorResult(result *mcp.CallToolResult) bool {
	if result == nil {
		return false
	}
	return result.IsError
}
