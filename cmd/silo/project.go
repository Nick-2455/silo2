package main

import (
	"strings"

	"github.com/nicolasperalta/silo2/internal/config"
)

// resolveProject decides which Engram project to use for the current
// command. Precedence (first non-empty wins):
//
//  1. --project CLI flag
//  2. config.Project from silo.config.json
//  3. config.DefaultProject ("silo2") — temporary dev fallback
//
// Whitespace-only values are treated as empty so a flag like
// `--project " "` does not silently override a meaningful config entry.
func resolveProject(flagVal, cfgVal string) string {
	if strings.TrimSpace(flagVal) != "" {
		return flagVal
	}
	if strings.TrimSpace(cfgVal) != "" {
		return cfgVal
	}
	return config.DefaultProject
}
