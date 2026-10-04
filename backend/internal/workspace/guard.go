package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func statFile(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// ErrOutsideProject is returned when a path would escape the project folder.
var ErrOutsideProject = fmt.Errorf("path is outside the project folder")

// SafePath resolves a path the model produced against the project root and
// refuses anything that lands outside it.
//
// This matters because an agent is told to write files, and a model can emit a
// path like "../../../../Windows/System32/config" or an absolute
// "C:\Users\me\.ssh\id_rsa". filepath.Join alone does not stop this: joining an
// absolute path simply discards the base, and a traversal walks straight out.
func SafePath(root, candidate string) (string, error) {
	cleanRoot := filepath.Clean(root)
	if cleanRoot == "" {
		return "", ErrOutsideProject
	}

	rel := strings.TrimSpace(candidate)
	if rel == "" {
		return "", fmt.Errorf("empty path")
	}

	// Reject paths with Windows volume names or absolute paths not rooted in cleanRoot
	if vol := filepath.VolumeName(rel); vol != "" {
		cleanCandidate := filepath.Clean(rel)
		lowerCandidate := strings.ToLower(cleanCandidate)
		lowerRoot := strings.ToLower(cleanRoot)
		if lowerCandidate == lowerRoot || strings.HasPrefix(lowerCandidate, lowerRoot+string(filepath.Separator)) {
			return cleanCandidate, nil
		}
		return "", fmt.Errorf("%w: %s", ErrOutsideProject, candidate)
	}

	// If candidate starts with a slash or backslash (drive-relative or root-relative on Unix)
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "\\") {
		cleanCandidate := filepath.Clean(rel)
		lowerCandidate := strings.ToLower(cleanCandidate)
		lowerRoot := strings.ToLower(cleanRoot)
		if lowerCandidate == lowerRoot || strings.HasPrefix(lowerCandidate, lowerRoot+string(filepath.Separator)) {
			return cleanCandidate, nil
		}
		return "", fmt.Errorf("%w: %s", ErrOutsideProject, candidate)
	}

	// Convert Windows separators so a model writing either style is handled.
	normalised := filepath.FromSlash(strings.ReplaceAll(rel, "\\", "/"))

	full := filepath.Clean(filepath.Join(cleanRoot, normalised))

	// The resolved path must be the root itself or sit under it.
	lowerFull := strings.ToLower(full)
	lowerRoot := strings.ToLower(cleanRoot)
	if lowerFull != lowerRoot && !strings.HasPrefix(lowerFull, lowerRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %s", ErrOutsideProject, candidate)
	}

	return full, nil
}

// ErrSecretAccess is returned when an agent attempts to access sensitive files.
var ErrSecretAccess = fmt.Errorf("access denied: reading secret or credential files is forbidden (PROJECT_VISION §25)")

// IsSecretPath reports whether a file path points to secrets or credentials
// that must never be read by or leaked to agents (PROJECT_VISION §25).
func IsSecretPath(path string) bool {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	base := strings.ToLower(filepath.Base(clean))
	// .env and its variants (.env.local, .env.production, etc.)
	if strings.HasPrefix(base, ".env") {
		return true
	}
	// SSH and cryptographic keys
	if strings.Contains(base, "id_rsa") || strings.Contains(base, "id_ecdsa") || strings.Contains(base, "id_ed25519") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(base))
	if ext == ".pem" || ext == ".key" || ext == ".p12" || ext == ".pfx" || ext == ".keystore" {
		return true
	}
	// Package manager credentials and cloud service tokens
	if base == ".npmrc" || base == ".pypirc" || base == ".netrc" || base == "credentials.json" {
		return true
	}
	if strings.HasPrefix(base, "service-account") && ext == ".json" {
		return true
	}
	return false
}

// SafeReadPath is SafePath plus an existence check, for reading files.
func SafeReadPath(root, candidate string) (string, error) {
	full, err := SafePath(root, candidate)
	if err != nil {
		return "", err
	}
	if rel, err := RelPath(root, full); err == nil && IsSecretPath(rel) {
		return "", fmt.Errorf("%w: %s", ErrSecretAccess, candidate)
	}
	info, err := statFile(full)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", candidate)
	}
	if info.IsDir() {
		return "", fmt.Errorf("not a file: %s", candidate)
	}
	return full, nil
}

// RelPath is the inverse of SafePath: it turns an absolute path back into a
// project-relative one for display and storage.
func RelPath(root, absolute string) (string, error) {
	cleanRoot := filepath.Clean(root)
	full := filepath.Clean(absolute)
	if full == cleanRoot {
		return "", nil
	}
	if !strings.HasPrefix(full, cleanRoot+string(filepath.Separator)) {
		return "", ErrOutsideProject
	}
	return filepath.ToSlash(strings.TrimPrefix(full, cleanRoot+string(filepath.Separator))), nil
}