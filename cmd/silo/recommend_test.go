package main

import (
	"bytes"
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

func TestRunRecommend_UsesFixedFreeMinutesWithoutSchedule(t *testing.T) {
	withTempWorkingDir(t)

	vaultDir := t.TempDir()
	writeRecommendConfig(t, vaultDir)
	writeRecommendProfile(t, vaultDir)
	writeRecommendSeed(t, vaultDir, "seed-abc.md", "Focus article")
	if err := os.WriteFile("schedule.json", []byte(`{"events":[{"duration_minutes":120,"days":["2026-06-01"]}]}`), 0o644); err != nil {
		t.Fatalf("WriteFile(schedule.json) error = %v", err)
	}

	output := captureStdout(t, func() {
		if err := runRecommend(nil); err != nil {
			t.Fatalf("runRecommend() error = %v", err)
		}
	})

	if !strings.Contains(output, "(8:00 libres)") {
		t.Fatalf("runRecommend() output = %q, want fixed 8 hours free", output)
	}
	if !strings.Contains(output, "Focus article") {
		t.Fatalf("runRecommend() output = %q, want open seed title", output)
	}
	if !strings.Contains(output, "Arquitectura") {
		t.Fatalf("runRecommend() output = %q, want profile focus", output)
	}
	if !strings.Contains(output, "Recomendaciones") {
		t.Fatalf("runRecommend() output = %q, want recommendation heading", output)
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
