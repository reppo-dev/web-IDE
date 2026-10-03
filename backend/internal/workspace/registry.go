package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Registry remembers where each project actually lives. The swarm refers to a
// project by a safe id, but the agents must read and write the real folder, so
// the mapping has to survive restarts.
type Registry struct {
	mu      sync.RWMutex
	dataDir string
	cache   map[string]ProjectRef
}

type registryFile struct {
	Projects []ProjectRef `json:"projects"`
}

// NewRegistry loads the project mapping from the data directory.
func NewRegistry(dataDir string) (*Registry, error) {
	r := &Registry{
		dataDir: dataDir,
		cache:   make(map[string]ProjectRef),
	}

	path := r.filePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil // first run
		}
		return nil, fmt.Errorf("failed to read project registry: %w", err)
	}

	var rf registryFile
	if err := json.Unmarshal(data, &rf); err != nil {
		// A corrupt registry must not stop the daemon; it is rebuilt as the
		// user opens projects again.
		return r, nil
	}
	for _, p := range rf.Projects {
		r.cache[p.ID] = p
	}
	return r, nil
}

func (r *Registry) filePath() string {
	return filepath.Join(r.dataDir, "projects.json")
}

// Register validates a path and remembers it, so the same folder always maps
// to the same id and keeps its memory.
func (r *Registry) Register(path string) (*ProjectRef, error) {
	ref, err := ResolveProject(path)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[ref.ID] = *ref
	if err := r.persistLocked(); err != nil {
		return nil, err
	}
	return ref, nil
}

// Lookup returns the folder for a project id.
func (r *Registry) Lookup(id string) (*ProjectRef, error) {
	r.mu.RLock()
	ref, ok := r.cache[id]
	r.mu.RUnlock()
	if ok {
		return &ref, nil
	}

	// The id may be a raw path (legacy callers), so try resolving it directly.
	if resolved, err := ResolveProject(id); err == nil {
		return resolved, nil
	}
	return nil, fmt.Errorf("project %q is not registered", id)
}

// List returns every registered project that still exists on disk.
func (r *Registry) List() []ProjectRef {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]ProjectRef, 0, len(r.cache))
	for _, p := range r.cache {
		if _, err := os.Stat(p.Root); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// RootFor returns the directory the agents should use for a project id.
func (r *Registry) RootFor(id string) (string, error) {
	ref, err := r.Lookup(id)
	if err != nil {
		return "", err
	}
	return ref.Root, nil
}

// Forget removes a project from the registry without touching the folder.
func (r *Registry) Forget(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.cache, id)
	return r.persistLocked()
}

func (r *Registry) persistLocked() error {
	if err := os.MkdirAll(r.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	rf := registryFile{Projects: make([]ProjectRef, 0, len(r.cache))}
	for _, p := range r.cache {
		rf.Projects = append(rf.Projects, p)
	}

	data, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode project registry: %w", err)
	}
	// Write to a temp file first so a crash mid-write cannot truncate the map.
	tmp := r.filePath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("failed to write project registry: %w", err)
	}
	if err := os.Rename(tmp, r.filePath()); err != nil {
		return fmt.Errorf("failed to replace project registry: %w", err)
	}
	return nil
}