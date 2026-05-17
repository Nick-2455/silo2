package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInboxCore_EmptyVault(t *testing.T) {
	var out bytes.Buffer
	err := inboxCore(t.TempDir(), &out)
	if err != nil {
		t.Fatalf("inboxCore: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "open       0") {
		t.Errorf("expected 'open 0' line, got:\n%s", s)
	}
	if !strings.Contains(s, "(no open seeds)") {
		t.Errorf("expected '(no open seeds)' hint, got:\n%s", s)
	}
}

func TestInboxCore_ListsOpenSeeds(t *testing.T) {
	dir := t.TempDir()
	writeSeed(t, dir, "Inbox/open/seed-aaa.md", "open")
	writeSeed(t, dir, "Inbox/open/seed-bbb.md", "open")
	writeSeed(t, dir, "Inbox/open/seed-ccc.md", "deferred")
	writeSeed(t, dir, "Inbox/archive/seed-ddd.md", "discarded")

	var out bytes.Buffer
	if err := inboxCore(dir, &out); err != nil {
		t.Fatal(err)
	}
	s := out.String()

	for _, want := range []string{
		"open       2",
		"deferred   1",
		"discarded  1",
		"seed-aaa.md",
		"seed-bbb.md",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	// seed-ccc has status deferred, so it must NOT appear under "Open seeds:".
	openIdx := strings.Index(s, "Open seeds:")
	if openIdx >= 0 && strings.Contains(s[openIdx:], "seed-ccc.md") {
		t.Errorf("seed-ccc.md should not be listed under Open seeds (status=deferred):\n%s", s)
	}
}

func writeSeed(t *testing.T, vault, rel, status string) {
	t.Helper()
	full := filepath.Join(vault, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\ntype: seed\nstatus: " + status + "\n---\n\n# T\n"
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
