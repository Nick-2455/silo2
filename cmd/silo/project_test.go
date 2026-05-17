package main

import (
	"testing"

	"github.com/nicolasperalta/silo2/internal/config"
)

func TestResolveProject_Precedence(t *testing.T) {
	cases := []struct {
		name, flag, cfg, want string
	}{
		{"flag wins over config", "from-flag", "from-cfg", "from-flag"},
		{"config wins over default", "", "from-cfg", "from-cfg"},
		{"default applies when both empty", "", "", config.DefaultProject},
		{"whitespace flag treated as empty", "   ", "from-cfg", "from-cfg"},
		{"whitespace config treated as empty", "", "  ", config.DefaultProject},
		{"whitespace flag and config", "  ", "  ", config.DefaultProject},
		{"flag trims surrounding spaces is not done", "  keep  ", "x", "  keep  "},
		// ↑ We only trim for the empty-check; if the caller passes a value
		// that survives trimming, we return it as-is (no surprise rewrites).
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := resolveProject(c.flag, c.cfg)
			if got != c.want {
				t.Errorf("resolveProject(%q, %q) = %q, want %q", c.flag, c.cfg, got, c.want)
			}
		})
	}
}

func TestDefaultProject_IsSilo2(t *testing.T) {
	if config.DefaultProject != "silo2" {
		t.Fatalf("DefaultProject changed unexpectedly: %q", config.DefaultProject)
	}
}
