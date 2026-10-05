package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexusai/nexusai/backend/internal/workspace"
)

// ProjectHandler opens real folders from disk and lists their files so the IDE
// can show a project the user already has.
type ProjectHandler struct {
	registry *workspace.Registry
}

// NewProjectHandler creates the handler. A nil registry disables the endpoints
// rather than crashing the daemon.
func NewProjectHandler(reg *workspace.Registry) *ProjectHandler {
	return &ProjectHandler{registry: reg}
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// HandleProjects serves:
//
//	GET    /api/projects                  list opened projects
//	POST   /api/projects                  open a folder {"path": "..."}
//	GET    /api/projects/{id}             one project's file tree
//	GET    /api/projects/{id}/file?path=  read one file
//	GET    /api/projects/{id}/plan        read project plan (.project/plan.json)
//	GET    /api/projects/{id}/ledger      read project change ledger (.project/ledger.jsonl)
//	POST   /api/projects/{id}/accept      accept and merge swarm checkpoints into active branch
func (h *ProjectHandler) HandleProjects(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "project registry unavailable"})
		return
	}

	path := r.URL.Path
	rest := strings.Trim(strings.TrimPrefix(path, "/api/projects"), "/")
	segments := strings.Split(rest, "/")
	id := ""
	sub := ""
	if len(segments) > 0 && segments[0] != "" {
		id = segments[0]
	}
	if len(segments) > 1 {
		sub = segments[1]
	}

	switch r.Method {
	case http.MethodGet:
		if id == "" {
			h.listProjects(w)
			return
		}
		if sub == "file" {
			h.readFile(w, r, id)
			return
		}
		if sub == "plan" {
			h.readPlan(w, r, id)
			return
		}
		if sub == "ledger" {
			h.readLedger(w, r, id)
			return
		}
		h.tree(w, r, id)

	case http.MethodPost:
		if id != "" && sub == "accept" {
			h.acceptChanges(w, r, id)
			return
		}
		h.openProject(w, r)

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *ProjectHandler) listProjects(w http.ResponseWriter) {
	projects := h.registry.List()
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	if projects == nil {
		projects = []workspace.ProjectRef{}
	}
	writeJSON(w, http.StatusOK, projects)
}

func (h *ProjectHandler) openProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}

	ref, err := h.registry.Register(body.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ref)
}

func (h *ProjectHandler) tree(w http.ResponseWriter, r *http.Request, id string) {
	ref, err := h.registry.Lookup(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	engine := workspace.NewContextEngine(ref.Root)
	files, err := engine.ListSourceFiles()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type entry struct {
		Path string `json:"path"`
		Size int    `json:"size"`
	}
	out := make([]entry, 0, len(files))
	for _, f := range files {
		out = append(out, entry{Path: f.RelPath, Size: f.Size})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":     ref.ID,
		"name":   ref.Name,
		"root":   ref.Root,
		"files":  out,
		"truncated": len(files) >= 500,
	})
}

// readFile serves one file's contents. The requested path is resolved inside
// the project root and rejected if it escapes, so a crafted request cannot
// read arbitrary files off the machine.
func (h *ProjectHandler) readFile(w http.ResponseWriter, r *http.Request, id string) {
	ref, err := h.registry.Lookup(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	rel := r.URL.Query().Get("path")
	if rel == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing path parameter"})
		return
	}

	full, err := safeJoin(ref.Root, rel)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "file not found"})
		return
	}
	// Refuse anything that is not source or config; the editor only needs text.
	if !workspace.IsViewableFile(full) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "file type not supported by the editor"})
		return
	}

	data, err := os.ReadFile(full)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Cap the response so a huge file cannot exhaust memory.
	const maxBytes = 2 * 1024 * 1024
	truncated := false
	if len(data) > maxBytes {
		data = data[:maxBytes]
		truncated = true
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":      filepath.ToSlash(rel),
		"language":  languageFor(full),
		"content":   string(data),
		"truncated": truncated,
	})
}

// readPlan serves the project's work plan from .project/plan.json.
func (h *ProjectHandler) readPlan(w http.ResponseWriter, r *http.Request, id string) {
	ref, err := h.registry.Lookup(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	plan, err := workspace.LoadPlan(ref.Root)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if plan == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"goal":  "",
			"tasks": []interface{}{},
		})
		return
	}

	writeJSON(w, http.StatusOK, plan)
}

// readLedger serves the project's change history from .project/ledger.jsonl.
func (h *ProjectHandler) readLedger(w http.ResponseWriter, r *http.Request, id string) {
	ref, err := h.registry.Lookup(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	entries, err := workspace.LoadProjectLedger(ref.Root)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if entries == nil {
		entries = []workspace.ChangeLedgerEntry{}
	}

	writeJSON(w, http.StatusOK, entries)
}

// acceptChanges merges the latest swarm checkpoint into the user's active branch.
// (PROJECT_VISION §22: Explicit approval required before modifying user's commit history).
func (h *ProjectHandler) acceptChanges(w http.ResponseWriter, r *http.Request, id string) {
	ref, err := h.registry.Lookup(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	ws, err := workspace.NewWorkspace("", ref.Root)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	commitHash, err := ws.AcceptChanges(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "accepted",
		"commit":     commitHash,
		"project_id": id,
	})
}

// safeJoin resolves rel against root and refuses any result outside root.
func safeJoin(root, rel string) (string, error) {
	cleanedRoot := filepath.Clean(root)
	target := filepath.Join(cleanedRoot, filepath.FromSlash(rel))

	// filepath.Join already cleans, but a symlink or an odd drive letter could
	// still land elsewhere, so compare the resolved absolute paths.
	if target != cleanedRoot && !strings.HasPrefix(target, cleanedRoot+string(filepath.Separator)) {
		return "", errors.New("path escapes the project folder")
	}
	return target, nil
}

func languageFor(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".md":
		return "markdown"
	case ".rs":
		return "rust"
	case ".sql":
		return "sql"
	case ".html":
		return "html"
	case ".css":
		return "css"
	default:
		return "plaintext"
	}
}