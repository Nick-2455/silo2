package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestConfig_DefaultLLMTimeoutZero(t *testing.T) {
	cfg := Default()

	if cfg.LLMTimeoutSeconds != 0 {
		t.Fatalf("Default().LLMTimeoutSeconds = %d, want 0", cfg.LLMTimeoutSeconds)
	}
	if got := cfg.SynthesisTimeout(); got != DefaultLLMTimeout {
		t.Fatalf("Default().SynthesisTimeout() = %v, want %v", got, DefaultLLMTimeout)
	}
	if DefaultLLMTimeout != 5*time.Second {
		t.Fatalf("DefaultLLMTimeout = %v, want 5s", DefaultLLMTimeout)
	}
}

func TestConfig_SynthesisTimeoutFromConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want time.Duration
	}{
		{name: "zero uses default", cfg: &Config{LLMTimeoutSeconds: 0}, want: 5 * time.Second},
		{name: "negative uses default", cfg: &Config{LLMTimeoutSeconds: -1}, want: 5 * time.Second},
		{name: "thirty seconds", cfg: &Config{LLMTimeoutSeconds: 30}, want: 30 * time.Second},
		{name: "one second", cfg: &Config{LLMTimeoutSeconds: 1}, want: time.Second},
		{name: "nil uses default", cfg: nil, want: 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.SynthesisTimeout(); got != tt.want {
				t.Fatalf("SynthesisTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

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

func TestConfig_LoadRoundTripIncludesTimeout(t *testing.T) {
	withTempWorkingDir(t)

	cfg := Default()
	cfg.LLMTimeoutSeconds = 30
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.LLMTimeoutSeconds != 30 {
		t.Fatalf("Load().LLMTimeoutSeconds = %d, want 30", got.LLMTimeoutSeconds)
	}
	if timeout := got.SynthesisTimeout(); timeout != 30*time.Second {
		t.Fatalf("Load().SynthesisTimeout() = %v, want 30s", timeout)
	}
}

func TestConfig_LoadOmitsTimeoutWhenAbsent(t *testing.T) {
	withTempWorkingDir(t)

	configJSON := `{
		"vault_path": "./vault",
		"llm_provider": "openai"
	}`
	if err := os.WriteFile(filepath.Join(".", Path()), []byte(configJSON), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.LLMTimeoutSeconds != 0 {
		t.Fatalf("LLMTimeoutSeconds = %d, want 0", got.LLMTimeoutSeconds)
	}
	if timeout := got.SynthesisTimeout(); timeout != 5*time.Second {
		t.Fatalf("SynthesisTimeout() = %v, want 5s", timeout)
	}
}

func TestConfig_DoesNotExposeSchedulePath(t *testing.T) {
	t.Parallel()

	if _, ok := reflect.TypeOf(Config{}).FieldByName("SchedulePath"); ok {
		t.Fatal("Config must not expose SchedulePath")
	}
}

func TestConfig_StillExposesProductiveHours(t *testing.T) {
	t.Parallel()

	field, ok := reflect.TypeOf(Config{}).FieldByName("ProductiveHours")
	if !ok {
		t.Fatal("Config must keep ProductiveHours")
	}
	if field.Type != reflect.TypeOf([][2]string{}) {
		t.Fatalf("ProductiveHours type = %v, want [][2]string", field.Type)
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
