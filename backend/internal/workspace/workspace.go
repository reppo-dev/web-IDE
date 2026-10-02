package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

// Workspace manages physical files on disk for a project, tracks diffs, and commits each round to Git.
type Workspace struct {
	baseDir   string
	projectID string
	projPath  string
	mu        sync.RWMutex
}

// NewWorkspace creates or attaches to a project directory.
func NewWorkspace(baseDir, projectID string) (*Workspace, error) {
	projPath := projectID
	if !filepath.IsAbs(projPath) {
		// Check if it exists as relative dir in current working dir
		if stat, err := os.Stat(projPath); err == nil && stat.IsDir() {
			abs, _ := filepath.Abs(projPath)
			projPath = abs
		} else {
			projPath = filepath.Join(baseDir, "workspaces", projectID)
		}
	}

	if err := os.MkdirAll(projPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	ws := &Workspace{
		baseDir:   baseDir,
		projectID: projectID,
		projPath:  projPath,
	}

	// Initialize git repo in workspace if not exists
	gitDir := filepath.Join(projPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		cmd := exec.Command("git", "init")
		cmd.Dir = projPath
		_ = cmd.Run()

		// Configure local git user if not present
		cfgName := exec.Command("git", "config", "user.name", "NexusAI Swarm")
		cfgName.Dir = projPath
		_ = cfgName.Run()

		cfgEmail := exec.Command("git", "config", "user.email", "swarm@nexusai.local")
		cfgEmail.Dir = projPath
		_ = cfgEmail.Run()
	}

	return ws, nil
}

// ProjectPath resolves where a project's workspace lives without creating it.
// Unlike NewWorkspace it performs no mkdir and no git init, so read-only
// callers such as the agents' context engine must use this instead.
func ProjectPath(baseDir, projectID string) string {
	if filepath.IsAbs(projectID) {
		return projectID
	}
	if stat, err := os.Stat(projectID); err == nil && stat.IsDir() {
		if abs, err := filepath.Abs(projectID); err == nil {
			return abs
		}
	}
	return filepath.Join(baseDir, "workspaces", projectID)
}

// Path returns the absolute path to this project workspace.
func (w *Workspace) Path() string {
	return w.projPath
}

// LoadFiles scans the workspace directory on disk and loads code files into domain.CodeFile.
func (w *Workspace) LoadFiles() ([]domain.CodeFile, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var files []domain.CodeFile
	ignoreDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		".next":        true,
		"dist":         true,
		"build":        true,
		".idea":        true,
		".vscode":      true,
		"__pycache__":   true,
		"target":       true,
		"bin":          true,
		"obj":          true,
	}

	langMap := map[string]string{
		".go":    "go",
		".ts":    "typescript",
		".tsx":   "typescript",
		".js":    "javascript",
		".jsx":   "javascript",
		".py":    "python",
		".html":  "html",
		".css":   "css",
		".json":  "json",
		".md":    "markdown",
		".yaml":  "yaml",
		".yml":   "yaml",
		".sql":   "sql",
		".sh":    "shell",
		".rs":    "rust",
		".java":  "java",
		".c":     "c",
		".cpp":   "cpp",
		".h":     "c",
		".toml":  "toml",
		".env":   "plaintext",
		".txt":   "plaintext",
	}

	err := filepath.Walk(w.projPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if ignoreDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Size() > 1024*1024 { // Skip > 1MB
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		baseName := filepath.Base(path)
		lang, ok := langMap[ext]
		if !ok && baseName != "Dockerfile" && baseName != "Makefile" {
			return nil
		}
		if !ok {
			lang = "plaintext"
		}

		relPath, err := filepath.Rel(w.projPath, path)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		files = append(files, domain.CodeFile{
			Path:     relPath,
			Language: lang,
			Content:  string(contentBytes),
		})
		return nil
	})

	return files, err
}

// SaveFile writes a single file to disk in the workspace.
func (w *Workspace) SaveFile(relPath, content string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := ValidateCodeFile(relPath, content); err != nil {
		return err
	}

	fullPath := filepath.Join(w.projPath, filepath.FromSlash(relPath))
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(content), 0644)
}

// SyncFiles writes all generated code files to disk and computes their diffs.
// Content that is not valid source is skipped with a reason rather than
// written, so a conversational answer never becomes a broken file.
func (w *Workspace) SyncFiles(files []domain.CodeFile) ([]domain.CodeFile, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var synced []domain.CodeFile

	for _, file := range files {
		if err := ValidateCodeFile(file.Path, file.Content); err != nil {
			// Keep the file in the list so the UI can show what was rejected,
			// but never write invalid content to disk.
			rejected := file
			rejected.Diff = "REJECTED: " + err.Error()
			synced = append(synced, rejected)
			continue
		}

		fullPath := filepath.Join(w.projPath, filepath.FromSlash(file.Path))
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for %s: %w", file.Path, err)
		}

		// Read old content if exists for diff
		oldContent := ""
		if oldBytes, err := os.ReadFile(fullPath); err == nil {
			oldContent = string(oldBytes)
		}

		// Write new content
		if err := os.WriteFile(fullPath, []byte(file.Content), 0644); err != nil {
			return nil, fmt.Errorf("failed to write file %s: %w", file.Path, err)
		}

		// Compute diff
		diff := computeSimpleDiff(file.Path, oldContent, file.Content)
		file.Diff = diff
		synced = append(synced, file)
	}

	return synced, nil
}

// CommitRound creates a Git commit for the round with Arbiter score metadata.
func (w *Workspace) CommitRound(ctx context.Context, round int, score float64, summary string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// git add .
	addCmd := exec.CommandContext(ctx, "git", "add", ".")
	addCmd.Dir = w.projPath
	if err := addCmd.Run(); err != nil {
		return "", fmt.Errorf("git add failed: %w", err)
	}

	// git commit
	msg := fmt.Sprintf("[NexusAI Swarm] Round %d | Score: %.1f%% | %s", round, score, summary)
	commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", msg, "--allow-empty")
	commitCmd.Dir = w.projPath
	out, err := commitCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git commit failed: %w (output: %s)", err, string(out))
	}

	// Get latest commit hash
	revCmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	revCmd.Dir = w.projPath
	hashBytes, _ := revCmd.Output()
	return strings.TrimSpace(string(hashBytes)), nil
}

// computeSimpleDiff creates a standard unified diff header representation.
func computeSimpleDiff(path, oldContent, newContent string) string {
	if oldContent == newContent {
		return ""
	}
	if oldContent == "" {
		return fmt.Sprintf("--- /dev/null\n+++ b/%s\n@@ -0,0 +1,%d @@\n+%s", path, len(strings.Split(newContent, "\n")), strings.ReplaceAll(newContent, "\n", "\n+"))
	}

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var diff strings.Builder
	diff.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", path, path))
	diff.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", len(oldLines), len(newLines)))

	max := len(oldLines)
	if len(newLines) > max {
		max = len(newLines)
	}

	for i := 0; i < max; i++ {
		if i < len(oldLines) && i < len(newLines) {
			if oldLines[i] == newLines[i] {
				diff.WriteString(" " + oldLines[i] + "\n")
			} else {
				diff.WriteString("-" + oldLines[i] + "\n")
				diff.WriteString("+" + newLines[i] + "\n")
			}
		} else if i < len(oldLines) {
			diff.WriteString("-" + oldLines[i] + "\n")
		} else if i < len(newLines) {
			diff.WriteString("+" + newLines[i] + "\n")
		}
	}

	return diff.String()
}
