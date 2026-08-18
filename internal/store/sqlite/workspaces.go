package sqlite

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/zand/atoms-demo/internal/domain"
	"github.com/zand/atoms-demo/internal/store"
)

const sessionLifetime = 365 * 24 * time.Hour

func (s *Store) InitializeWorkspace(ctx context.Context, displayName string) (domain.Workspace, string, error) {
	normalizedName, err := domain.NormalizeDisplayName(displayName)
	if err != nil {
		return domain.Workspace{}, "", err
	}

	workspaceID, err := newID("ws")
	if err != nil {
		return domain.Workspace{}, "", err
	}
	token, err := newSessionToken()
	if err != nil {
		return domain.Workspace{}, "", err
	}

	now := s.now().UTC()
	workspace := domain.Workspace{
		ID:          workspaceID,
		DisplayName: normalizedName,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	tokenHash := sha256.Sum256([]byte(token))

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Workspace{}, "", fmt.Errorf("begin workspace transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(
		ctx,
		"INSERT INTO workspaces(id, display_name, created_at, updated_at) VALUES (?, ?, ?, ?)",
		workspace.ID,
		workspace.DisplayName,
		timestamp(workspace.CreatedAt),
		timestamp(workspace.UpdatedAt),
	); err != nil {
		return domain.Workspace{}, "", fmt.Errorf("insert workspace: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		"INSERT INTO sessions(token_hash, workspace_id, expires_at, created_at) VALUES (?, ?, ?, ?)",
		tokenHash[:],
		workspace.ID,
		timestamp(now.Add(sessionLifetime)),
		timestamp(now),
	); err != nil {
		return domain.Workspace{}, "", fmt.Errorf("insert session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Workspace{}, "", fmt.Errorf("commit workspace transaction: %w", err)
	}

	return workspace, token, nil
}

func (s *Store) WorkspaceBySession(ctx context.Context, token string) (domain.Workspace, error) {
	if token == "" {
		return domain.Workspace{}, store.ErrSessionNotFound
	}
	tokenHash := sha256.Sum256([]byte(token))

	var workspace domain.Workspace
	var createdAt, updatedAt string
	err := s.db.QueryRowContext(
		ctx,
		`
			SELECT w.id, w.display_name, w.created_at, w.updated_at
			FROM sessions AS s
			JOIN workspaces AS w ON w.id = s.workspace_id
			WHERE s.token_hash = ? AND s.expires_at > ?
		`,
		tokenHash[:],
		timestamp(s.now()),
	).Scan(&workspace.ID, &workspace.DisplayName, &createdAt, &updatedAt)
	if isNoRows(err) {
		return domain.Workspace{}, store.ErrSessionNotFound
	}
	if err != nil {
		return domain.Workspace{}, fmt.Errorf("lookup workspace by session: %w", err)
	}

	workspace.CreatedAt, err = parseTimestamp(createdAt)
	if err != nil {
		return domain.Workspace{}, err
	}
	workspace.UpdatedAt, err = parseTimestamp(updatedAt)
	if err != nil {
		return domain.Workspace{}, err
	}
	return workspace, nil
}
