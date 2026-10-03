package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/nexusai/nexusai/backend/internal/domain"
	"github.com/nexusai/nexusai/backend/internal/team"
)

// TeamHandler exposes HTTP endpoints for managing custom teams and agents.
type TeamHandler struct {
	store team.Store
}

// NewTeamHandler creates a new TeamHandler.
func NewTeamHandler(store team.Store) *TeamHandler {
	return &TeamHandler{store: store}
}

// HandleTeams handles GET (list or single), POST (create/update), and DELETE.
func (h *TeamHandler) HandleTeams(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.store == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "team store unavailable"})
		return
	}

	// The subtree pattern also matches /api/teams/default, which belongs to the
	// dedicated handler. Without this, asking for the default would look up a
	// team whose id happens to be "default".
	if strings.TrimSuffix(r.URL.Path, "/") == "/api/teams/default" {
		h.HandleDefaultTeam(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id == "" && strings.HasPrefix(r.URL.Path, "/api/teams/") {
			id = strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/teams/"), "/")
		}
		if id != "" {
			t, err := h.store.Get(r.Context(), id)
			if err != nil {
				if errors.Is(err, domain.ErrTeamNotFound) {
					w.WriteHeader(http.StatusNotFound)
					json.NewEncoder(w).Encode(map[string]string{"error": "team not found"})
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			json.NewEncoder(w).Encode(t)
			return
		}

		teams, err := h.store.List(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		if teams == nil {
			teams = []*domain.Team{}
		}
		json.NewEncoder(w).Encode(teams)

	case http.MethodPost:
		var t domain.Team
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid json payload: " + err.Error()})
			return
		}

		if err := h.store.Save(r.Context(), &t); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(t)

	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" && strings.HasPrefix(r.URL.Path, "/api/teams/") {
			id = strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/teams/"), "/")
		}
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing id parameter"})
			return
		}

		if err := h.store.Delete(r.Context(), id); err != nil {
			if errors.Is(err, domain.ErrTeamNotFound) {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "team not found"})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": id})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleDefaultTeam handles GET (retrieve default team) and POST (designate default team).
func (h *TeamHandler) HandleDefaultTeam(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.store == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "team store unavailable"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		def, err := h.store.GetDefault(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(def)

	case http.MethodPost:
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request, 'id' required"})
			return
		}

		if err := h.store.SetDefault(r.Context(), req.ID); err != nil {
			if errors.Is(err, domain.ErrTeamNotFound) {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "team not found"})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "default_updated", "id": req.ID})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
