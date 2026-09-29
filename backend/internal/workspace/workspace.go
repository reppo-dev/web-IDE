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
	projPath := filepath.Join(baseDir, "workspaces", projectID)
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

// Path returns the absolute path to this project workspace.
func (w *Workspace) Path() string {
	return w.projPath
}

// SyncFiles writes all generated code files to disk and computes their diffs.
func (w *Workspace) SyncFiles(files []domain.CodeFile) ([]domain.CodeFile, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var synced []domain.CodeFile

	for _, file := range files {
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
