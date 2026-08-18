package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zand/atoms-demo/internal/domain"
	"github.com/zand/atoms-demo/internal/store"
)

func (s *Store) ListProjects(ctx context.Context, workspaceID string) ([]domain.Project, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`
			SELECT id, workspace_id, name, summary, active_version_id, created_at, updated_at
			FROM projects
			WHERE workspace_id = ?
			ORDER BY updated_at DESC, id DESC
		`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]domain.Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return projects, nil
}

func (s *Store) CreateProject(ctx context.Context, workspaceID, name string) (domain.Project, error) {
	normalizedName, err := domain.NormalizeProjectName(name)
	if err != nil {
		return domain.Project{}, err
	}
	projectID, err := newID("prj")
	if err != nil {
		return domain.Project{}, err
	}
	now := s.now().UTC()
	project := domain.Project{
		ID:          projectID,
		WorkspaceID: workspaceID,
		Name:        normalizedName,
		Summary:     "",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if _, err := s.db.ExecContext(
		ctx,
		`
			INSERT INTO projects(id, workspace_id, name, summary, active_version_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, NULL, ?, ?)
		`,
		project.ID,
		project.WorkspaceID,
		project.Name,
		project.Summary,
		timestamp(project.CreatedAt),
		timestamp(project.UpdatedAt),
	); err != nil {
		return domain.Project{}, fmt.Errorf("create project: %w", err)
	}
	return project, nil
}

func (s *Store) ProjectByID(ctx context.Context, workspaceID, projectID string) (domain.Project, error) {
	return s.projectByID(ctx, workspaceID, projectID)
}

func (s *Store) RenameProject(ctx context.Context, workspaceID, projectID, name string) (domain.Project, error) {
	normalizedName, err := domain.NormalizeProjectName(name)
	if err != nil {
		return domain.Project{}, err
	}
	now := s.now().UTC()
	result, err := s.db.ExecContext(
		ctx,
		"UPDATE projects SET name = ?, updated_at = ? WHERE id = ? AND workspace_id = ?",
		normalizedName,
		timestamp(now),
		projectID,
		workspaceID,
	)
	if err != nil {
		return domain.Project{}, fmt.Errorf("rename project: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return domain.Project{}, fmt.Errorf("check renamed project: %w", err)
	}
	if changed == 0 {
		return domain.Project{}, store.ErrProjectNotFound
	}
	return s.projectByID(ctx, workspaceID, projectID)
}

func (s *Store) DeleteProject(ctx context.Context, workspaceID, projectID string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM projects WHERE id = ? AND workspace_id = ?", projectID, workspaceID)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted project: %w", err)
	}
	if changed == 0 {
		return store.ErrProjectNotFound
	}
	return nil
}

func (s *Store) projectByID(ctx context.Context, workspaceID, projectID string) (domain.Project, error) {
	row := s.db.QueryRowContext(
		ctx,
		`
			SELECT id, workspace_id, name, summary, active_version_id, created_at, updated_at
			FROM projects
			WHERE id = ? AND workspace_id = ?
		`,
		projectID,
		workspaceID,
	)
	project, err := scanProject(row)
	if isNoRows(err) {
		return domain.Project{}, store.ErrProjectNotFound
	}
	if err != nil {
		return domain.Project{}, err
	}
	return project, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(row rowScanner) (domain.Project, error) {
	var project domain.Project
	var activeVersionID sql.NullString
	var createdAt, updatedAt string
	if err := row.Scan(
		&project.ID,
		&project.WorkspaceID,
		&project.Name,
		&project.Summary,
		&activeVersionID,
		&createdAt,
		&updatedAt,
	); err != nil {
		return domain.Project{}, err
	}
	if activeVersionID.Valid {
		project.ActiveVersionID = new(activeVersionID.String)
	}
	var err error
	project.CreatedAt, err = parseTimestamp(createdAt)
	if err != nil {
		return domain.Project{}, err
	}
	project.UpdatedAt, err = parseTimestamp(updatedAt)
	if err != nil {
		return domain.Project{}, err
	}
	return project, nil
}
