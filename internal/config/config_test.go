package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultsKeepLLMFieldsOptional(t *testing.T) {
	withTempWorkingDir(t)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.LLMProvider != "" || got.LLMModel != "" || got.LLMAPIKey != "" {
		t.Fatalf("expected empty optional llm fields, got %#v", got)
	}
}

func TestLoad_ReadsOptionalLLMFields(t *testing.T) {
	withTempWorkingDir(t)

	configJSON := `{
		"llm_provider": "openai",
		"llm_model": "gpt-4.1-mini",
		"llm_api_key": "test-key"
	}`
	if err := os.WriteFile(filepath.Join(".", Path()), []byte(configJSON), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.LLMProvider != "openai" || got.LLMModel != "gpt-4.1-mini" || got.LLMAPIKey != "test-key" {
		t.Fatalf("Load() llm fields mismatch: %#v", got)
	}
}

func TestSave_PersistsOptionalLLMFields(t *testing.T) {
	withTempWorkingDir(t)

	cfg := Default()
	cfg.LLMProvider = "openai"
	cfg.LLMModel = "gpt-4.1-mini"
	cfg.LLMAPIKey = "test-key"

	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if got.LLMProvider != cfg.LLMProvider || got.LLMModel != cfg.LLMModel || got.LLMAPIKey != cfg.LLMAPIKey {
		t.Fatalf("roundtrip mismatch: want %#v got %#v", cfg, got)
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
