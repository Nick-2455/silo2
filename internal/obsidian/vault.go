package obsidian

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Vault struct {
	Path string
}

func NewVault(path string) *Vault {
	return &Vault{Path: path}
}

func (v *Vault) EnsureDir() error {
	if v == nil {
		return errors.New("vault is nil")
	}
	if v.Path == "" {
		return errors.New("vault path is empty")
	}
	return os.MkdirAll(v.Path, 0o755)
}

func (v *Vault) Exists() bool {
	if v == nil || v.Path == "" {
		return false
	}
	st, err := os.Stat(v.Path)
	if err != nil {
		return false
	}
	return st.IsDir()
}

func (v *Vault) WriteNote(name string, content string) error {
	return v.WriteNoteAt("", name, content)
}

// WriteResult tells the caller what WriteNoteIfAbsent decided to do. This
// is the single source of truth for the "do not overwrite human edits"
// policy: anyone wanting that guarantee MUST go through WriteNoteIfAbsent.
type WriteResult int

const (
	// Written means the file did not exist and was created.
	Written WriteResult = iota
	// Skipped means the file already existed and was left untouched.
	Skipped
)

// WriteNoteIfAbsent writes <vault>/<subdir>/<name> ONLY if no file exists
// at that path. If the file is already there it is left byte-for-byte
// untouched and Skipped is returned. This is what protects curated notes
// from being trampled on repeated `silo curate` runs.
//
// Any non-os.ErrNotExist stat error is surfaced — we never overwrite on
// ambiguous filesystem state.
func (v *Vault) WriteNoteIfAbsent(subdir, name, content string) (WriteResult, error) {
	if v == nil {
		return Skipped, errors.New("vault is nil")
	}
	if v.Path == "" {
		return Skipped, errors.New("vault path is empty")
	}
	if name == "" {
		return Skipped, errors.New("note name is empty")
	}
	if err := validateRelPath(name); err != nil {
		return Skipped, fmt.Errorf("invalid note name %q: %w", name, err)
	}
	if subdir != "" {
		if err := validateRelPath(subdir); err != nil {
			return Skipped, fmt.Errorf("invalid subdir %q: %w", subdir, err)
		}
	}

	dir := v.Path
	if subdir != "" {
		dir = filepath.Join(v.Path, filepath.FromSlash(subdir))
	}
	target := filepath.Join(dir, name)

	// Containment check before anything touches disk.
	absRoot, err := filepath.Abs(v.Path)
	if err != nil {
		return Skipped, err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return Skipped, err
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil || strings.HasPrefix(rel, "..") {
		return Skipped, fmt.Errorf("refused write outside vault: %s", target)
	}

	if _, err := os.Stat(target); err == nil {
		return Skipped, nil
	} else if !os.IsNotExist(err) {
		return Skipped, fmt.Errorf("stat %s: %w", target, err)
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Skipped, err
	}
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return Skipped, err
	}
	return Written, nil
}

// WriteNoteAt writes a note under <vault>/<subdir>/<name>, creating the
// subdirectory if needed. Both subdir and name are validated to prevent
// any escape outside the vault root (no absolute paths, no "..").
func (v *Vault) WriteNoteAt(subdir, name, content string) error {
	if v == nil {
		return errors.New("vault is nil")
	}
	if v.Path == "" {
		return errors.New("vault path is empty")
	}
	if name == "" {
		return errors.New("note name is empty")
	}
	if err := validateRelPath(name); err != nil {
		return fmt.Errorf("invalid note name %q: %w", name, err)
	}
	if subdir != "" {
		if err := validateRelPath(subdir); err != nil {
			return fmt.Errorf("invalid subdir %q: %w", subdir, err)
		}
	}

	dir := v.Path
	if subdir != "" {
		dir = filepath.Join(v.Path, filepath.FromSlash(subdir))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	p := filepath.Join(dir, name)
	// Final containment check after Join resolves any embedded separators.
	absRoot, err := filepath.Abs(v.Path)
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(p)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("refused write outside vault: %s", p)
	}

	return os.WriteFile(p, []byte(content), 0o644)
}

func validateRelPath(p string) error {
	if p == "" {
		return errors.New("empty")
	}
	if filepath.IsAbs(p) {
		return errors.New("absolute path not allowed")
	}
	// Reject parent-traversal segments in any form.
	parts := strings.Split(filepath.ToSlash(p), "/")
	for _, part := range parts {
		if part == ".." {
			return errors.New("parent traversal not allowed")
		}
	}
	return nil
}
