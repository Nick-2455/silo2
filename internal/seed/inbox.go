package seed

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Inbox layout (intentionally tiny):
//
//   <vault>/Inbox/
//     open/      <-- fresh seeds awaiting triage
//     archive/   <-- everything the human is done thinking about
//
// State lives in the seed's frontmatter (`status: open | deferred |
// discarded | approved`), NOT in folder names. This keeps the filesystem
// from drifting into a task-state-machine and leaves room for any future
// status without a directory migration.
//
// Promotion semantics are deliberately absent from this package: moving
// a seed to archive/, or flipping its `status:` field, is a human act.
// Silo only counts and lists what it sees.

// InboxStatus enumerates the MVP statuses recognized in seed frontmatter.
// Unknown values are bucketed into Other so we never lose visibility.
type InboxStatus string

const (
	StatusOpen      InboxStatus = "open"
	StatusDeferred  InboxStatus = "deferred"
	StatusDiscarded InboxStatus = "discarded"
	StatusApproved  InboxStatus = "approved"
)

// InboxScan is the result of one filesystem walk over <vault>/Inbox/.
// Counts are by status; OpenFiles lists the filenames currently sitting
// in <vault>/Inbox/open/ with status == open, sorted alphabetically so
// CLI output is deterministic across runs.
type InboxScan struct {
	Open      int
	Deferred  int
	Discarded int
	Approved  int
	Other     int

	OpenFiles []string
}

func (s InboxScan) Total() int {
	return s.Open + s.Deferred + s.Discarded + s.Approved + s.Other
}

// ScanInbox walks <vault>/Inbox/open and <vault>/Inbox/archive and
// returns counts by status. It does not require the directories to
// exist — a vault with no captures yet returns a zero-value scan.
func ScanInbox(vaultPath string) (InboxScan, error) {
	if strings.TrimSpace(vaultPath) == "" {
		return InboxScan{}, errors.New("vault path is empty")
	}

	var scan InboxScan
	for _, sub := range []string{"Inbox/open", "Inbox/archive"} {
		dir := filepath.Join(vaultPath, sub)
		if err := walkSeeds(dir, &scan); err != nil {
			return InboxScan{}, fmt.Errorf("scan %s: %w", sub, err)
		}
	}

	sort.Strings(scan.OpenFiles)
	return scan, nil
}

func walkSeeds(dir string, scan *InboxScan) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if name == "README.md" || strings.EqualFold(name, "readme.md") {
			continue
		}

		full := filepath.Join(dir, name)
		status, err := readSeedStatus(full)
		if err != nil {
			// Skip unreadable files rather than crashing the whole scan.
			// The user can fix the file by hand; the rest of the inbox
			// must remain visible.
			continue
		}

		switch InboxStatus(strings.ToLower(strings.TrimSpace(status))) {
		case StatusOpen:
			scan.Open++
			if filepath.Base(dir) == "open" {
				scan.OpenFiles = append(scan.OpenFiles, name)
			}
		case StatusDeferred:
			scan.Deferred++
		case StatusDiscarded:
			scan.Discarded++
		case StatusApproved:
			scan.Approved++
		default:
			scan.Other++
		}
	}
	return nil
}

// readSeedStatus pulls the `status:` value from the first YAML
// frontmatter block. Missing frontmatter or missing field defaults to
// "open" so hand-authored notes still show up in triage.
func readSeedStatus(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	// First non-empty line must be "---" to count as frontmatter.
	if !sc.Scan() {
		return string(StatusOpen), nil
	}
	if strings.TrimSpace(sc.Text()) != "---" {
		return string(StatusOpen), nil
	}

	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		if k, v, ok := splitYAMLLine(line); ok && k == "status" {
			return v, nil
		}
	}
	return string(StatusOpen), nil
}

func splitYAMLLine(line string) (key, val string, ok bool) {
	i := strings.IndexByte(line, ':')
	if i <= 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
}
