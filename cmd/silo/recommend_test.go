package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nicolasperalta/silo2/internal/config"
)

func TestRunRecommend_RejectsRemovedDateFlag(t *testing.T) {
	withTempWorkingDir(t)
	writeRecommendConfig(t, t.TempDir())

	err := runRecommend([]string{"-date", "2026-06-01"})
	if err == nil {
		t.Fatal("expected removed -date flag to return an error")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("runRecommend() error = %v, want unknown flag error", err)
	}
}

func TestRunRecommend_UsesFreeMinutesFlagAndPrintsJSON(t *testing.T) {
	withTempWorkingDir(t)

	vaultDir := t.TempDir()
	writeRecommendConfig(t, vaultDir)
	writeRecommendProfile(t, vaultDir)
	writeRecommendSeed(t, vaultDir, "seed-abc.md", "Focus article")

	output := captureStdout(t, func() {
		if err := runRecommend([]string{"-free-minutes", "120"}); err != nil {
			t.Fatalf("runRecommend() error = %v", err)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &payload); err != nil {
		t.Fatalf("Unmarshal(output) error = %v; output=%q", err, output)
	}
	if payload["free_minutes"] != float64(120) {
		t.Fatalf("free_minutes = %v, want 120", payload["free_minutes"])
	}
	if payload["seeds_considered"] != float64(1) {
		t.Fatalf("seeds_considered = %v, want 1", payload["seeds_considered"])
	}
	recs, ok := payload["recommendations"].([]any)
	if !ok || len(recs) != 1 {
		t.Fatalf("recommendations = %v, want one recommendation", payload["recommendations"])
	}
	rec, ok := recs[0].(map[string]any)
	if !ok {
		t.Fatalf("recommendation[0] = %T, want object", recs[0])
	}
	if rec["title"] != "Focus article" {
		t.Fatalf("recommendation title = %v, want Focus article", rec["title"])
	}
}

func writeRecommendConfig(t *testing.T, vaultDir string) {
	t.Helper()
	configJSON := "{\n  \"vault_path\": " + quoteJSON(vaultDir) + "\n}\n"
	if err := os.WriteFile(config.Path(), []byte(configJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
}

func withTempWorkingDir(t *testing.T) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("Chdir(%q) error = %v", tmp, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working dir: %v", err)
		}
	})
}

func writeRecommendProfile(t *testing.T, vaultDir string) {
	t.Helper()
	path := filepath.Join(vaultDir, "Silo", "Profile.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(profile) error = %v", err)
	}
	content := "---\n{\"current_focus\":[\"Arquitectura\"]}\n---\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(profile) error = %v", err)
	}
}

func writeRecommendSeed(t *testing.T, vaultDir, name, title string) {
	t.Helper()
	path := filepath.Join(vaultDir, "Inbox", "open", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(seed) error = %v", err)
	}
	if err := os.WriteFile(path, []byte("# "+title+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(seed) error = %v", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })

	outC := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outC <- buf.String()
	}()

	fn()
	_ = w.Close()
	return <-outC
}

func quoteJSON(s string) string {
	var buf bytes.Buffer
	buf.WriteByte('"')
	for _, r := range s {
		if r == '\\' || r == '"' {
			buf.WriteByte('\\')
		}
		buf.WriteRune(r)
	}
	buf.WriteByte('"')
	return buf.String()
}
