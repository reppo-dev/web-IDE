package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectRef is a validated pointer to a project directory on disk.
type ProjectRef struct {
	// ID is a filesystem-safe key derived from the path. It is used for the
	// memory database and workspace folder, never as a raw path.
	ID string `json:"id"`
	// Name is the folder name, shown in the UI.
	Name string `json:"name"`
	// Root is the absolute path the agents will read and write.
	Root string `json:"root"`
}

// ResolveProject turns a user-supplied path into a safe ProjectRef. Only
// directories that exist are accepted, and the resulting ID never contains a
// path separator so it cannot escape the data directory.
func ResolveProject(path string) (*ProjectRef, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, fmt.Errorf("project path is empty")
	}

	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid project path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("folder does not exist: %s", abs)
		}
		return nil, fmt.Errorf("cannot read folder: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a folder: %s", abs)
	}

	name := filepath.Base(abs)
	if name == "." || name == string(filepath.Separator) {
		name = "project"
	}

	return &ProjectRef{
		ID:   ProjectIDFor(abs),
		Name: name,
		Root: abs,
	}, nil
}

// ProjectIDFor derives a stable, filesystem-safe id from an absolute path.
// Two calls with the same path always produce the same id, so a project keeps
// its memory across sessions.
func ProjectIDFor(absPath string) string {
	// Windows paths are case-insensitive, so normalise before hashing or the
	// same folder would get two different memories.
	normalised := filepath.Clean(absPath)
	if len(normalised) > 1 && normalised[1] == ':' {
		normalised = strings.ToLower(normalised)
	}

	sum := sha256.Sum256([]byte(normalised))
	hash := hex.EncodeToString(sum[:])[:12]

	// Keep a readable prefix so the data folder is not just hashes.
	slug := slugify(filepath.Base(filepath.Clean(absPath)))
	if slug == "" {
		slug = "project"
	}
	return slug + "-" + hash
}

// slugify reduces a folder name to lowercase letters, digits and dashes.
func slugify(s string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}