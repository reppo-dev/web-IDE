package team

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nexusai/nexusai/backend/internal/domain"
	_ "modernc.org/sqlite"
)

// Store provides persistence for custom teams and agent configurations.
type Store interface {
	Save(ctx context.Context, team *domain.Team) error
	Get(ctx context.Context, id string) (*domain.Team, error)
	List(ctx context.Context) ([]*domain.Team, error)
	Delete(ctx context.Context, id string) error
	SetDefault(ctx context.Context, id string) error
	GetDefault(ctx context.Context) (*domain.Team, error)
	Close() error
}

// SQLiteStore stores teams in a SQLite database.
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore opens or creates the teams SQLite database in dataDir.
func NewSQLiteStore(dataDir string) (*SQLiteStore, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create team data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "teams.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open teams db: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *SQLiteStore) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS teams (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		description TEXT,
		data TEXT NOT NULL,
		is_default INTEGER DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_teams_updated_at ON teams(updated_at DESC);
	`
	_, err := s.db.Exec(query)
	return err
}

// Save validates and persists a team. If ID is empty, a UUID is generated.
func (s *SQLiteStore) Save(ctx context.Context, team *domain.Team) error {
	if team == nil {
		return domain.ErrTeamNil
	}

	if team.ID == "" {
		team.ID = uuid.New().String()
	}

	now := time.Now()
	if team.CreatedAt.IsZero() {
		team.CreatedAt = now
	}
	team.UpdatedAt = now

	if err := team.Validate(); err != nil {
		return err
	}

	payload, err := team.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize team: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
	INSERT INTO teams (id, name, description, data, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		name = excluded.name,
		description = excluded.description,
		data = excluded.data,
		updated_at = excluded.updated_at;
	`
	_, err = s.db.ExecContext(ctx, query, team.ID, team.Name, team.Description, payload, team.CreatedAt, team.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to write team: %w", err)
	}
	return nil
}

// Get retrieves a team by its ID.
func (s *SQLiteStore) Get(ctx context.Context, id string) (*domain.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data string
	query := `SELECT data FROM teams WHERE id = ?`
	err := s.db.QueryRowContext(ctx, query, id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, domain.ErrTeamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query team: %w", err)
	}

	return domain.TeamFromJSON(data)
}

// List returns all stored teams, sorted newest first.
func (s *SQLiteStore) List(ctx context.Context) ([]*domain.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT data FROM teams ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}
	defer rows.Close()

	var out []*domain.Team
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		t, err := domain.TeamFromJSON(data)
		if err != nil {
			continue
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Delete removes a team by ID.
func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM teams WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return domain.ErrTeamNotFound
	}
	return nil
}

// SetDefault designates a team as the default team.
func (s *SQLiteStore) SetDefault(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `UPDATE teams SET is_default = 0`); err != nil {
		return err
	}

	res, err := tx.ExecContext(ctx, `UPDATE teams SET is_default = 1 WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return domain.ErrTeamNotFound
	}

	return tx.Commit()
}

// GetDefault returns the active default team or the baseline 5-agent team.
func (s *SQLiteStore) GetDefault(ctx context.Context) (*domain.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data string
	query := `SELECT data FROM teams WHERE is_default = 1 LIMIT 1`
	err := s.db.QueryRowContext(ctx, query).Scan(&data)
	if err == nil {
		return domain.TeamFromJSON(data)
	}

	// Fallback to built-in default team
	return DefaultTeam(), nil
}

// DefaultTeam produces the canonical 5-agent team.
func DefaultTeam() *domain.Team {
	return &domain.Team{
		ID:                 "default",
		Name:               "NexusAI Core Swarm",
		Description:        "Standard 5-agent software engineering team",
		ConsensusThreshold: 85.0,
		MaxRounds:          3,
		Agents: []domain.AgentSpec{
			{
				ID:      "architect",
				Name:    "System Architect",
				Role:    domain.RoleArchitect,
				Model:   "flash_5",
				Phase:   domain.PhaseDesign,
				Enabled: true,
			},
			{
				ID:      "coder",
				Name:    "Lead Engineer",
				Role:    domain.RoleCoder,
				Model:   "mimo",
				Phase:   domain.PhaseImplement,
				Enabled: true,
			},
			{
				ID:      "critic",
				Name:    "Adversarial Critic",
				Role:    domain.RoleCritic,
				Model:   "bunny",
				Phase:   domain.PhaseReview,
				Enabled: true,
			},
			{
				ID:      "security",
				Name:    "Security Auditor",
				Role:    domain.RoleSecurity,
				Model:   "flash_5",
				Phase:   domain.PhaseReview,
				Enabled: true,
			},
			{
				ID:      "arbiter",
				Name:    "Arbiter Judge",
				Role:    domain.RoleArbiter,
				Model:   "bunny",
				Phase:   domain.PhaseJudge,
				Enabled: true,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// DevelopersTeam returns the permanent 5-agent team of developers requested by user:
// nemo_qwen, nemotron, bunny, flash_5, mimo without specialized roles.
func DevelopersTeam() *domain.Team {
	return &domain.Team{
		ID:                 "team-developers-ninerouter",
		Name:               "تیم دولوپر ها",
		Description:        "تیم دائمی ۵ نفره توسعه‌دهندگان بر پایه ۹Router بدون نقش اختصاصی",
		ConsensusThreshold: 85.0,
		MaxRounds:          3,
		Agents: []domain.AgentSpec{
			{
				ID:      "dev-nemo-qwen",
				Name:    "Nemo Qwen",
				Role:    domain.RoleCustom,
				Model:   "nemo_qwen",
				Phase:   domain.PhaseImplement,
				Enabled: true,
			},
			{
				ID:      "dev-nemotron",
				Name:    "Nemotron",
				Role:    domain.RoleCustom,
				Model:   "nemotron",
				Phase:   domain.PhaseImplement,
				Enabled: true,
			},
			{
				ID:      "dev-bunny",
				Name:    "Bunny",
				Role:    domain.RoleCustom,
				Model:   "bunny",
				Phase:   domain.PhaseImplement,
				Enabled: true,
			},
			{
				ID:      "dev-flash-5",
				Name:    "Flash 5",
				Role:    domain.RoleCustom,
				Model:   "flash_5",
				Phase:   domain.PhaseImplement,
				Enabled: true,
			},
			{
				ID:      "dev-mimo",
				Name:    "MiMo",
				Role:    domain.RoleCustom,
				Model:   "mimo",
				Phase:   domain.PhaseImplement,
				Enabled: true,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// SeedDevelopersTeam persists the default developers team if not already present.
func (s *SQLiteStore) SeedDevelopersTeam() error {
	t := DevelopersTeam()
	payload, err := t.ToJSON()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `
	INSERT OR IGNORE INTO teams (id, name, description, data, is_default, created_at, updated_at)
	VALUES (?, ?, ?, ?, 0, ?, ?);
	`
	_, err = s.db.Exec(query, t.ID, t.Name, t.Description, payload, t.CreatedAt, t.UpdatedAt)
	return err
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}