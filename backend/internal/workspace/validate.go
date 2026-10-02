package workspace

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateCodeFile rejects content that is obviously not source code before it
// reaches disk. A model that answers a request conversationally can otherwise
// write its prose into main.go, which is how "User Safety: safe" ended up in a
// Go source file.
func ValidateCodeFile(path, content string) error {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return fmt.Errorf("%s is empty", path)
	}

	// A prose answer is short, has no code punctuation, and reads as a sentence.
	// Real source of any language has at least one structural character.
	hasStructure := strings.ContainsAny(trimmed, "{}();=<>[]")
	isProse := !hasStructure &&
		strings.ContainsAny(trimmed, " .,!؟") &&
		len(trimmed) < 2000

	if isProse {
		return fmt.Errorf("%s contains prose, not source code: %q", path, truncate(trimmed, 80))
	}

	// Go source must declare a package. Without it the file cannot compile, and
	// writing it only produces a broken workspace.
	if strings.EqualFold(filepath.Ext(path), ".go") && !strings.Contains(trimmed, "package ") {
		return fmt.Errorf("%s has no Go package declaration", path)
	}

	// JSON must at least start like an object or array.
	if strings.EqualFold(filepath.Ext(path), ".json") {
		if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
			return fmt.Errorf("%s is not valid JSON", path)
		}
	}

	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}