package config

import (
	"encoding/json"
	"errors"
	"os"
)

const defaultConfigPath = "./silo.config.json"

type Config struct {
	VaultPath      string `json:"vault_path"`
	EngramEndpoint string `json:"engram_endpoint,omitempty"`
	EngramAPIKey   string `json:"engram_api_key,omitempty"`
	IdentityName   string `json:"identity_name,omitempty"`
	LLMProvider    string `json:"llm_provider,omitempty"`
	LLMModel       string `json:"llm_model,omitempty"`
	LLMAPIKey      string `json:"llm_api_key,omitempty"`

	// Project is the Engram project name Silo will read from. If empty,
	// the CLI falls back to DefaultProject. This default exists only for
	// local development convenience and may become required in the future.
	Project string `json:"project,omitempty"`
}

// DefaultProject is the temporary fallback project name used when neither
// the --project flag nor the config provides one. It will likely become
// required (or inferred from the working directory) in a later milestone.
const DefaultProject = "silo2"

func Path() string {
	return defaultConfigPath
}

func Exists() bool {
	_, err := os.Stat(Path())
	return err == nil
}

func Default() *Config {
	return &Config{
		VaultPath:      "./vault",
		EngramEndpoint: "",
		EngramAPIKey:   "",
		IdentityName:   "",
		LLMProvider:    "",
		LLMModel:       "",
		LLMAPIKey:      "",
		Project:        "",
	}
}

func Load() (*Config, error) {
	b, err := os.ReadFile(Path())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// MVP behavior: missing config falls back to defaults.
			return Default(), nil
		}
		return nil, err
	}

	cfg := Default()
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, err
	}
	if cfg.VaultPath == "" {
		return nil, errors.New("vault_path must not be empty")
	}
	return cfg, nil
}

func Save(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if cfg.VaultPath == "" {
		return errors.New("vault_path must not be empty")
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(Path(), b, 0o644)
}
