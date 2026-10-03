package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/nexusai/nexusai/backend/internal/domain"
)

// Workspace manages physical files on disk for a project, tracks diffs, and commits each round to Git.
type Workspace struct {
	baseDir   string
	projectID string
	projPath  string
	brain     *BrainFS
	mu        sync.RWMutex
}

// NewWorkspace creates or attaches to a project directory.
func NewWorkspace(baseDir, projectID string) (*Workspace, error) {
	projPath := ProjectPath(baseDir, projectID)

	if err := os.MkdirAll(projPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	ws := &Workspace{
		baseDir:   baseDir,
		projectID: projectID,
		projPath:  projPath,
	}

	// Initialize .project/ Project Brain filesystem (PROJECT_VISION §8)
	if b, err := NewBrainFS(projPath); err == nil {
		ws.brain = b
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

// registry is set once at startup so ProjectPath can map a safe project id back
// to the folder the user actually opened. It is nil in tests that never
// register a project.
var registry atomic.Pointer[Registry]

// SetRegistry installs the process-wide project registry.
func SetRegistry(r *Registry) {
	registry.Store(r)
}

// ProjectPath resolves where a project's workspace lives without creating it.
// A registered project id maps to the folder the user opened; an unknown id
// falls back to a sandbox folder under the data directory.
func ProjectPath(baseDir, projectID string) string {
	if projectID == "" {
		return filepath.Join(baseDir, "workspaces", "default")
	}

	// A registered id always wins, so the swarm works on the real project.
	if r := registry.Load(); r != nil {
		if root, err := r.RootFor(projectID); err == nil {
			return root
		}
	}

	// A raw path is still accepted so callers can open a folder directly.
	if filepath.IsAbs(projectID) {
		return projectID
	}
	if stat, err := os.Stat(projectID); err == nil && stat.IsDir() {
		if abs, err := filepath.Abs(projectID); err == nil {
			return abs
		}
	}

	// An unknown slug is a generated project: keep it inside the sandbox.
	return filepath.Join(baseDir, "workspaces", SanitizeID(projectID))
}

// SanitizeID strips anything that could escape a directory name.
func SanitizeID(id string) string {
	cleaned := slugify(id)
	if cleaned == "" {
		return "default"
	}
	return cleaned
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
		".project":     true,
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

	// An agent must never write outside the folder the user opened.
	fullPath, err := SafePath(w.projPath, relPath)
	if err != nil {
		return err
	}

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
		// Anything outside the project folder is refused before validation:
		// a model that emits "../../.ssh/id_rsa" must never be written.
		fullPath, pathErr := SafePath(w.projPath, file.Path)
		if pathErr != nil {
			rejected := file
			rejected.Diff = "REJECTED: " + pathErr.Error()
			synced = append(synced, rejected)
			continue
		}

		if err := ValidateCodeFile(file.Path, file.Content); err != nil {
			// Keep the file in the list so the UI can show what was rejected,
			// but never write invalid content to disk.
			rejected := file
			rejected.Diff = "REJECTED: " + err.Error()
			synced = append(synced, rejected)
			continue
		}

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

	revCmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	revCmd.Dir = w.projPath
	hashBytes, _ := revCmd.Output()
	hash := strings.TrimSpace(string(hashBytes))

	// Record in append-only Change Ledger and update state.json (PROJECT_VISION §8)
	if w.brain != nil {
		_ = w.brain.AppendLedger(
			"round_committed",
			"Orchestrator",
			fmt.Sprintf("Round %d committed with score %.1f%% (%s)", round, score, summary),
			hash,
		)
		_ = w.brain.UpdateState(func(s *ProjectState) {
			s.Round = round
			s.LastCheckpoint = hash
			if score >= 85.0 {
				s.TaskStatus = TaskDone
			} else {
				s.TaskStatus = TaskRework
				s.ReworkCount++
			}
		})
	}

	return hash, nil
}

// Brain returns the project's .project/ BrainFS interface.
func (w *Workspace) Brain() *BrainFS {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.brain
}

// RollbackToCheckpoint resets the workspace git state to a specific commit hash.
func (w *Workspace) RollbackToCheckpoint(ctx context.Context, commitHash string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	cleaned := strings.TrimSpace(commitHash)
	if cleaned == "" {
		return fmt.Errorf("empty commit hash for rollback")
	}

	cmd := exec.CommandContext(ctx, "git", "reset", "--hard", cleaned)
	cmd.Dir = w.projPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset --hard failed: %w (output: %s)", err, string(out))
	}
	return nil
}

// RollbackUncommitted discards all unstaged or uncommitted working tree modifications.
func (w *Workspace) RollbackUncommitted(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	cmd := exec.CommandContext(ctx, "git", "checkout", ".")
	cmd.Dir = w.projPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout . failed: %w (output: %s)", err, string(out))
	}

	cleanCmd := exec.CommandContext(ctx, "git", "clean", "-fd")
	cleanCmd.Dir = w.projPath
	if out, err := cleanCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clean -fd failed: %w (output: %s)", err, string(out))
	}

	return nil
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
