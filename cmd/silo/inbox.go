package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/seed"
)

// `silo inbox` is the W2 surface, MVP edition.
//
// Today: a flat textual listing of counts by status plus the names of
// open seeds. No TUI, no filtering, no actions. The promotion act is
// human — done by editing the seed's `status:` field or moving it to
// Inbox/archive/.
//
// Why so thin: the value of W2 is the human ritual, not the interface.
// A real TUI is a separate change once the ritual proves itself.

func runInbox(args []string) error {
	fs := flag.NewFlagSet("inbox", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.VaultPath == "" {
		return errors.New("vault_path is empty in config")
	}
	return inboxCore(cfg.VaultPath, os.Stdout)
}

// inboxCore is the testable core. Kept separate from runInbox so tests
// drive it with a tmp vault and a captured writer.
func inboxCore(vaultPath string, out io.Writer) error {
	scan, err := seed.ScanInbox(vaultPath)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Inbox: %s/Inbox\n\n", vaultPath)
	fmt.Fprintf(out, "open       %d\n", scan.Open)
	fmt.Fprintf(out, "deferred   %d\n", scan.Deferred)
	fmt.Fprintf(out, "discarded  %d\n", scan.Discarded)
	fmt.Fprintf(out, "approved   %d\n", scan.Approved)
	if scan.Other > 0 {
		fmt.Fprintf(out, "other      %d\n", scan.Other)
	}

	fmt.Fprintln(out)
	if len(scan.OpenFiles) == 0 {
		fmt.Fprintln(out, "(no open seeds)")
		return nil
	}
	fmt.Fprintln(out, "Open seeds:")
	for _, name := range scan.OpenFiles {
		fmt.Fprintf(out, "  %s\n", name)
	}
	return nil
}
