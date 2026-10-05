package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nexusai/nexusai/backend/internal/domain"
	_ "modernc.org/sqlite"
)

// SQLiteMemoryStore implements MemoryStore using local SQLite databases.
type SQLiteMemoryStore struct {
	baseDir    string
	globalDB   *sql.DB
	projectDBs map[string]*sql.DB
	mu         sync.RWMutex
}

// NewSQLiteMemoryStore initializes the memory directory and connects to global DB.
func NewSQLiteMemoryStore(baseDir string) (*SQLiteMemoryStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base memory dir: %w", err)
	}

	globalPath := filepath.Join(baseDir, "global.db")
	db, err := sql.Open("sqlite", globalPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open global db: %w", err)
	}

	store := &SQLiteMemoryStore{
		baseDir:    baseDir,
		globalDB:   db,
		projectDBs: make(map[string]*sql.DB),
	}

	if err := store.initGlobalSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteMemoryStore) initGlobalSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS global_memories (
		id          TEXT PRIMARY KEY,
		type        TEXT NOT NULL,
		content     TEXT NOT NULL,
		source_proj TEXT,
		tags        TEXT,
		confidence  REAL DEFAULT 1.0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_global_type ON global_memories(type);
	`
	_, err := s.globalDB.Exec(query)
	return err
}

func (s *SQLiteMemoryStore) getProjectDB(projectID string) (*sql.DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if db, exists := s.projectDBs[projectID]; exists {
		return db, nil
	}

	projDir := filepath.Join(s.baseDir, "projects", projectID)
	if err := os.MkdirAll(projDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project memory dir: %w", err)
	}

	dbPath := filepath.Join(projDir, "memories.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open project db: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS memories (
		id          TEXT PRIMARY KEY,
		project_id  TEXT NOT NULL,
		agent       TEXT NOT NULL,
		type        TEXT NOT NULL,
		content     TEXT NOT NULL,
		evidence    TEXT,
		tags        TEXT,
		round       INTEGER DEFAULT 0,
		confidence  REAL DEFAULT 1.0,
		expires_at  DATETIME,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_memories_agent ON memories(agent);
	CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init project schema: %w", err)
	}

	// Dynamic column migrations for existing databases
	_, _ = db.Exec("ALTER TABLE memories ADD COLUMN evidence TEXT;")
	_, _ = db.Exec("ALTER TABLE memories ADD COLUMN expires_at DATETIME;")

	s.projectDBs[projectID] = db
	return db, nil
}

// WriteProject saves or updates a memory in the project's SQLite DB.
func (s *SQLiteMemoryStore) WriteProject(ctx context.Context, entry domain.MemoryEntry) error {
	if entry.ProjectID == "" {
		return fmt.Errorf("project_id is required")
	}
	if entry.ID == "" {
		entry.ID = "mem-" + uuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	entry.UpdatedAt = time.Now()

	db, err := s.getProjectDB(entry.ProjectID)
	if err != nil {
		return err
	}

	tagsJSON, _ := json.Marshal(entry.Tags)

	var expiresAtVal interface{}
	if entry.ExpiresAt != nil && !entry.ExpiresAt.IsZero() {
		expiresAtVal = entry.ExpiresAt.UTC().Format("2006-01-02 15:04:05")
	}

	query := `
	INSERT INTO memories (id, project_id, agent, type, content, evidence, tags, round, confidence, expires_at, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		content = excluded.content,
		evidence = excluded.evidence,
		tags = excluded.tags,
		confidence = excluded.confidence,
		expires_at = excluded.expires_at,
		updated_at = excluded.updated_at
	`
	_, err = db.ExecContext(ctx, query,
		entry.ID, entry.ProjectID, string(entry.Agent), string(entry.Type),
		entry.Content, entry.Evidence, string(tagsJSON), entry.Round, entry.Confidence,
		expiresAtVal, entry.CreatedAt, entry.UpdatedAt,
	)
	return err
}

// QueryProject queries memories from the project DB.
func (s *SQLiteMemoryStore) QueryProject(ctx context.Context, projectID string, filter QueryFilter) ([]domain.MemoryEntry, error) {
	db, err := s.getProjectDB(projectID)
	if err != nil {
		return nil, err
	}

	var conditions []string
	var args []interface{}

	if filter.Agent != "" {
		conditions = append(conditions, "agent = ?")
		args = append(args, string(filter.Agent))
	}
	if filter.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, string(filter.Type))
	}
	if filter.MinScore > 0 {
		conditions = append(conditions, "confidence >= ?")
		args = append(args, filter.MinScore)
	}
	if filter.Tag != "" {
		conditions = append(conditions, "tags LIKE ?")
		args = append(args, "%"+filter.Tag+"%")
	}

	// Exclude expired memories (PROJECT_VISION §10)
	conditions = append(conditions, "(expires_at IS NULL OR expires_at > datetime('now'))")

	query := "SELECT id, project_id, agent, type, content, COALESCE(evidence, ''), tags, round, confidence, expires_at, created_at, updated_at FROM memories"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY updated_at DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.MemoryEntry
	for rows.Next() {
		var m domain.MemoryEntry
		var agentStr, typeStr, tagsStr string
		var evidenceStr string
		var expiresAtRaw sql.NullString
		var createdAtStr, updatedAtStr string
		err := rows.Scan(
			&m.ID, &m.ProjectID, &agentStr, &typeStr, &m.Content,
			&evidenceStr, &tagsStr, &m.Round, &m.Confidence,
			&expiresAtRaw, &createdAtStr, &updatedAtStr,
		)
		if err != nil {
			return nil, err
		}
		m.Agent = domain.AgentRole(agentStr)
		m.Type = domain.MemoryType(typeStr)
		m.Evidence = evidenceStr
		_ = json.Unmarshal([]byte(tagsStr), &m.Tags)
		if expiresAtRaw.Valid && expiresAtRaw.String != "" {
			t := parseTime(expiresAtRaw.String)
			m.ExpiresAt = &t
		}
		m.CreatedAt = parseTime(createdAtStr)
		m.UpdatedAt = parseTime(updatedAtStr)
		results = append(results, m)
	}

	return results, nil
}

// WriteGlobal saves a cross-project memory entry.
func (s *SQLiteMemoryStore) WriteGlobal(ctx context.Context, entry domain.MemoryEntry) error {
	if entry.ID == "" {
		entry.ID = "gmem-" + uuid.New().String()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	tagsJSON, _ := json.Marshal(entry.Tags)

	query := `
	INSERT INTO global_memories (id, type, content, source_proj, tags, confidence, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		content = excluded.content,
		tags = excluded.tags,
		confidence = excluded.confidence
	`
	_, err := s.globalDB.ExecContext(ctx, query,
		entry.ID, string(entry.Type), entry.Content, entry.ProjectID,
		string(tagsJSON), entry.Confidence, entry.CreatedAt,
	)
	return err
}

// QueryGlobal queries cross-project memories.
func (s *SQLiteMemoryStore) QueryGlobal(ctx context.Context, filter QueryFilter) ([]domain.MemoryEntry, error) {
	var conditions []string
	var args []interface{}

	if filter.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, string(filter.Type))
	}
	if filter.MinScore > 0 {
		conditions = append(conditions, "confidence >= ?")
		args = append(args, filter.MinScore)
	}
	if filter.Tag != "" {
		conditions = append(conditions, "tags LIKE ?")
		args = append(args, "%"+filter.Tag+"%")
	}

	query := "SELECT id, type, content, source_proj, tags, confidence, created_at FROM global_memories"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC"
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := s.globalDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.MemoryEntry
	for rows.Next() {
		var m domain.MemoryEntry
		var typeStr, tagsStr, sourceProj string
		var createdAtStr string
		err := rows.Scan(
			&m.ID, &typeStr, &m.Content, &sourceProj,
			&tagsStr, &m.Confidence, &createdAtStr,
		)
		if err != nil {
			return nil, err
		}
		m.Type = domain.MemoryType(typeStr)
		m.ProjectID = sourceProj
		_ = json.Unmarshal([]byte(tagsStr), &m.Tags)
		m.CreatedAt = parseTime(createdAtStr)
		results = append(results, m)
	}

	return results, nil
}

func parseTime(raw string) time.Time {
	if raw == "" {
		return time.Now()
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, raw); err == nil {
			return t
		}
	}
	return time.Now()
}

// DeleteMemory deletes a memory by ID.
func (s *SQLiteMemoryStore) DeleteMemory(ctx context.Context, id string, isGlobal bool) error {
	if isGlobal {
		_, err := s.globalDB.ExecContext(ctx, "DELETE FROM global_memories WHERE id = ?", id)
		return err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, db := range s.projectDBs {
		_, err := db.ExecContext(ctx, "DELETE FROM memories WHERE id = ?", id)
		if err == nil {
			return nil
		}
	}
	return nil
}

// Close closes all database handles.
func (s *SQLiteMemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var firstErr error
	if s.globalDB != nil {
		if err := s.globalDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for _, db := range s.projectDBs {
		if err := db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
