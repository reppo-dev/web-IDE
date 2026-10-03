package agent

import "github.com/nexusai/nexusai/backend/internal/workspace"

// ContextEngine assembles the real project state that an agent needs before it
// writes code. It delegates to workspace.ContextEngine.
type ContextEngine = workspace.ContextEngine

// ProjectFile is one file discovered on disk, relative to the project root.
type ProjectFile = workspace.ProjectFile

// NewContextEngine builds a context engine rooted at the project directory.
func NewContextEngine(root string) *ContextEngine {
	return workspace.NewContextEngine(root)
}

// IsViewableFile reports whether the editor may open a file.
func IsViewableFile(path string) bool {
	return workspace.IsViewableFile(path)
}