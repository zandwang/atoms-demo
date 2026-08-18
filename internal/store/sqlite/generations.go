package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/zand/atoms-demo/internal/domain"
	"github.com/zand/atoms-demo/internal/store"
)

const maxStoredMessages = 100

func (s *Store) markInterruptedGenerations(ctx context.Context) error {
	now := s.now().UTC()
	if _, err := s.db.ExecContext(ctx, `
		UPDATE generation_attempts
		SET status = 'interrupted', error_code = 'INTERRUPTED',
		    error_message = '上次生成未完成，可重新提交需求。', completed_at = ?
		WHERE status = 'running'
	`, timestamp(now)); err != nil {
		return fmt.Errorf("mark interrupted generations: %w", err)
	}
	return nil
}

func (s *Store) ListMessages(ctx context.Context, workspaceID, projectID string, limit int) ([]domain.Message, error) {
	if _, err := s.projectByID(ctx, workspaceID, projectID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > maxStoredMessages {
		limit = maxStoredMessages
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, role, content, generation_id, created_at
		FROM (
			SELECT id, project_id, role, content, generation_id, created_at
			FROM messages
			WHERE project_id = ?
			ORDER BY created_at DESC, id DESC
			LIMIT ?
		)
		ORDER BY created_at ASC, id ASC
	`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	messages := make([]domain.Message, 0)
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate messages: %w", err)
	}
	return messages, nil
}

func (s *Store) ListVersions(ctx context.Context, workspaceID, projectID string) ([]domain.GenerationVersion, error) {
	if _, err := s.projectByID(ctx, workspaceID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, parent_version_id, sequence, request, plan_json, spec_json,
		       artifact_entry_html, artifact_html, artifact_css, artifact_js,
		       artifact_manifest_json, checksum, created_at
		FROM versions
		WHERE project_id = ?
		ORDER BY sequence DESC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	defer rows.Close()

	versions := make([]domain.GenerationVersion, 0)
	for rows.Next() {
		version, err := scanGenerationVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate versions: %w", err)
	}
	return versions, nil
}

func (s *Store) ActiveVersion(ctx context.Context, workspaceID, projectID string) (*domain.GenerationVersion, error) {
	project, err := s.projectByID(ctx, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	if project.ActiveVersionID == nil {
		return nil, nil
	}
	version, err := s.versionByID(ctx, projectID, *project.ActiveVersionID)
	if err != nil {
		return nil, err
	}
	return &version, nil
}

func (s *Store) BeginGeneration(ctx context.Context, workspaceID, projectID, request string) (domain.GenerationAttempt, error) {
	normalizedRequest, err := domain.NormalizeGenerationRequest(request)
	if err != nil {
		return domain.GenerationAttempt{}, err
	}
	attemptID, err := newID("gen")
	if err != nil {
		return domain.GenerationAttempt{}, err
	}
	messageID, err := newID("msg")
	if err != nil {
		return domain.GenerationAttempt{}, err
	}
	now := s.now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.GenerationAttempt{}, fmt.Errorf("begin generation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureProjectInWorkspace(ctx, tx, workspaceID, projectID); err != nil {
		return domain.GenerationAttempt{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO messages(id, project_id, role, content, generation_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, messageID, projectID, domain.MessageRoleUser, normalizedRequest, attemptID, timestamp(now)); err != nil {
		return domain.GenerationAttempt{}, fmt.Errorf("insert user generation message: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO generation_attempts(id, project_id, status, error_code, error_message, started_at, completed_at)
		VALUES (?, ?, 'running', NULL, NULL, ?, NULL)
	`, attemptID, projectID, timestamp(now)); err != nil {
		return domain.GenerationAttempt{}, fmt.Errorf("insert generation attempt: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE projects SET updated_at = ? WHERE id = ?", timestamp(now), projectID); err != nil {
		return domain.GenerationAttempt{}, fmt.Errorf("touch project for generation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.GenerationAttempt{}, fmt.Errorf("commit generation start: %w", err)
	}
	return domain.GenerationAttempt{ID: attemptID, ProjectID: projectID, Status: "running", StartedAt: now}, nil
}

func (s *Store) CompleteGeneration(ctx context.Context, workspaceID, projectID, attemptID string, expectedActiveVersionID *string, result domain.AgentResult, artifact domain.CompiledArtifact) (domain.GenerationVersion, error) {
	if err := result.NormalizeAndValidate(); err != nil {
		return domain.GenerationVersion{}, err
	}
	if err := validateArtifact(artifact, result.Spec.TemplateName()); err != nil {
		return domain.GenerationVersion{}, err
	}
	planJSON, err := json.Marshal(result.Plan)
	if err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("encode generation plan: %w", err)
	}
	specJSON, err := json.Marshal(result.Spec)
	if err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("encode generation spec: %w", err)
	}
	manifestJSON, err := json.Marshal(artifact.Manifest)
	if err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("encode artifact manifest: %w", err)
	}
	versionID, err := newID("ver")
	if err != nil {
		return domain.GenerationVersion{}, err
	}
	messageID, err := newID("msg")
	if err != nil {
		return domain.GenerationVersion{}, err
	}
	now := s.now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("begin generation completion transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var activeVersionID sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT active_version_id FROM projects WHERE id = ? AND workspace_id = ?
	`, projectID, workspaceID).Scan(&activeVersionID); err != nil {
		if isNoRows(err) {
			return domain.GenerationVersion{}, store.ErrProjectNotFound
		}
		return domain.GenerationVersion{}, fmt.Errorf("load project for generation completion: %w", err)
	}

	var request string
	if err := tx.QueryRowContext(ctx, `
		SELECT content FROM messages
		WHERE project_id = ? AND generation_id = ? AND role = ?
		ORDER BY created_at ASC, id ASC
		LIMIT 1
	`, projectID, attemptID, domain.MessageRoleUser).Scan(&request); err != nil {
		if isNoRows(err) {
			return domain.GenerationVersion{}, store.ErrGenerationAttemptNotFound
		}
		return domain.GenerationVersion{}, fmt.Errorf("load generation request: %w", err)
	}

	var sequence int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(sequence), 0) + 1 FROM versions WHERE project_id = ?", projectID).Scan(&sequence); err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("next version sequence: %w", err)
	}
	parentVersionID := cloneOptionalID(expectedActiveVersionID)

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO versions(
			id, project_id, parent_version_id, sequence, request, plan_json, spec_json,
			artifact_entry_html, artifact_html, artifact_css, artifact_js,
			artifact_manifest_json, checksum, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, versionID, projectID, parentVersionID, sequence, request, string(planJSON), string(specJSON),
		artifact.EntryHTML, artifact.HTML, artifact.CSS, artifact.JS, string(manifestJSON), artifact.Manifest.Checksum, timestamp(now)); err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("insert generation version: %w", err)
	}

	updated, err := tx.ExecContext(ctx, `
		UPDATE generation_attempts
		SET status = 'completed', error_code = NULL, error_message = NULL, completed_at = ?
		WHERE id = ? AND project_id = ? AND status = 'running'
	`, timestamp(now), attemptID, projectID)
	if err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("complete generation attempt: %w", err)
	}
	changed, err := updated.RowsAffected()
	if err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("check generation completion: %w", err)
	}
	if changed == 0 {
		return domain.GenerationVersion{}, store.ErrGenerationAttemptNotFound
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO messages(id, project_id, role, content, generation_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, messageID, projectID, domain.MessageRoleAssistant, result.AssistantMessage, attemptID, timestamp(now)); err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("insert assistant generation message: %w", err)
	}
	if sameOptionalID(activeVersionID, expectedActiveVersionID) {
		if _, err := tx.ExecContext(ctx, `
			UPDATE projects
			SET active_version_id = ?, summary = ?, updated_at = ?
			WHERE id = ? AND workspace_id = ?
		`, versionID, result.Plan.Summary, timestamp(now), projectID, workspaceID); err != nil {
			return domain.GenerationVersion{}, fmt.Errorf("activate completed version: %w", err)
		}
	} else if _, err := tx.ExecContext(ctx, "UPDATE projects SET updated_at = ? WHERE id = ? AND workspace_id = ?", timestamp(now), projectID, workspaceID); err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("touch project after non-active generation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("commit generation completion: %w", err)
	}

	return domain.GenerationVersion{
		ID:              versionID,
		ProjectID:       projectID,
		ParentVersionID: parentVersionID,
		Sequence:        sequence,
		Request:         request,
		Plan:            result.Plan,
		Spec:            result.Spec,
		Artifact:        artifact,
		CreatedAt:       now,
	}, nil
}

func cloneOptionalID(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func sameOptionalID(current sql.NullString, expected *string) bool {
	if expected == nil {
		return !current.Valid
	}
	return current.Valid && current.String == *expected
}

func (s *Store) FailGeneration(ctx context.Context, workspaceID, projectID, attemptID, errorCode, message string) error {
	if _, err := domain.NormalizeGenerationRequest(message); err != nil {
		return fmt.Errorf("validate failure message: %w", err)
	}
	messageID, err := newID("msg")
	if err != nil {
		return err
	}
	now := s.now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin generation failure transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureProjectInWorkspace(ctx, tx, workspaceID, projectID); err != nil {
		return err
	}
	updated, err := tx.ExecContext(ctx, `
		UPDATE generation_attempts
		SET status = 'failed', error_code = ?, error_message = ?, completed_at = ?
		WHERE id = ? AND project_id = ? AND status = 'running'
	`, errorCode, message, timestamp(now), attemptID, projectID)
	if err != nil {
		return fmt.Errorf("fail generation attempt: %w", err)
	}
	changed, err := updated.RowsAffected()
	if err != nil {
		return fmt.Errorf("check failed generation attempt: %w", err)
	}
	if changed == 0 {
		return store.ErrGenerationAttemptNotFound
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO messages(id, project_id, role, content, generation_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, messageID, projectID, domain.MessageRoleSystem, message, attemptID, timestamp(now)); err != nil {
		return fmt.Errorf("insert generation failure message: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "UPDATE projects SET updated_at = ? WHERE id = ?", timestamp(now), projectID); err != nil {
		return fmt.Errorf("touch project for generation failure: %w", err)
	}
	return tx.Commit()
}

func (s *Store) ActivateVersion(ctx context.Context, workspaceID, projectID, versionID string) (domain.Project, error) {
	now := s.now().UTC()
	messageID, err := newID("msg")
	if err != nil {
		return domain.Project{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Project{}, fmt.Errorf("begin version activation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := ensureProjectInWorkspace(ctx, tx, workspaceID, projectID); err != nil {
		return domain.Project{}, err
	}
	var sequence int
	var planJSON string
	if err := tx.QueryRowContext(ctx, "SELECT sequence, plan_json FROM versions WHERE id = ? AND project_id = ?", versionID, projectID).Scan(&sequence, &planJSON); err != nil {
		if isNoRows(err) {
			return domain.Project{}, store.ErrProjectNotFound
		}
		return domain.Project{}, fmt.Errorf("load version for activation: %w", err)
	}
	var plan domain.AgentPlan
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		return domain.Project{}, fmt.Errorf("decode version plan for activation: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE projects SET active_version_id = ?, summary = ?, updated_at = ?
		WHERE id = ? AND workspace_id = ?
	`, versionID, plan.Summary, timestamp(now), projectID, workspaceID); err != nil {
		return domain.Project{}, fmt.Errorf("activate version: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO messages(id, project_id, role, content, generation_id, created_at)
		VALUES (?, ?, ?, ?, NULL, ?)
	`, messageID, projectID, domain.MessageRoleSystem, fmt.Sprintf("已恢复到版本 v%d。", sequence), timestamp(now)); err != nil {
		return domain.Project{}, fmt.Errorf("record version activation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.Project{}, fmt.Errorf("commit version activation: %w", err)
	}
	return s.projectByID(ctx, workspaceID, projectID)
}

func (s *Store) PreviewState(ctx context.Context, workspaceID, projectID, versionID string) (json.RawMessage, error) {
	if _, err := s.projectByID(ctx, workspaceID, projectID); err != nil {
		return nil, err
	}
	if _, err := s.versionByID(ctx, projectID, versionID); err != nil {
		return nil, err
	}
	var state string
	err := s.db.QueryRowContext(ctx, "SELECT state_json FROM preview_states WHERE project_id = ? AND version_id = ?", projectID, versionID).Scan(&state)
	if isNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load preview state: %w", err)
	}
	return json.RawMessage(state), nil
}

func (s *Store) SavePreviewState(ctx context.Context, workspaceID, projectID, versionID string, state json.RawMessage) error {
	if _, err := s.projectByID(ctx, workspaceID, projectID); err != nil {
		return err
	}
	version, err := s.versionByID(ctx, projectID, versionID)
	if err != nil {
		return err
	}
	normalizedState, err := domain.NormalizePreviewState(version.Spec, state)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO preview_states(project_id, version_id, state_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(project_id, version_id) DO UPDATE SET state_json = excluded.state_json, updated_at = excluded.updated_at
	`, projectID, versionID, string(normalizedState), timestamp(now)); err != nil {
		return fmt.Errorf("save preview state: %w", err)
	}
	return nil
}

func (s *Store) versionByID(ctx context.Context, projectID, versionID string) (domain.GenerationVersion, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, parent_version_id, sequence, request, plan_json, spec_json,
		       artifact_entry_html, artifact_html, artifact_css, artifact_js,
		       artifact_manifest_json, checksum, created_at
		FROM versions
		WHERE id = ? AND project_id = ?
	`, versionID, projectID)
	version, err := scanGenerationVersion(row)
	if isNoRows(err) {
		return domain.GenerationVersion{}, store.ErrProjectNotFound
	}
	if err != nil {
		return domain.GenerationVersion{}, err
	}
	return version, nil
}

func ensureProjectInWorkspace(ctx context.Context, tx *sql.Tx, workspaceID, projectID string) error {
	var foundID string
	err := tx.QueryRowContext(ctx, "SELECT id FROM projects WHERE id = ? AND workspace_id = ?", projectID, workspaceID).Scan(&foundID)
	if isNoRows(err) {
		return store.ErrProjectNotFound
	}
	if err != nil {
		return fmt.Errorf("verify project workspace: %w", err)
	}
	return nil
}

func scanMessage(row rowScanner) (domain.Message, error) {
	var message domain.Message
	var generationID sql.NullString
	var createdAt string
	if err := row.Scan(&message.ID, &message.ProjectID, &message.Role, &message.Content, &generationID, &createdAt); err != nil {
		return domain.Message{}, err
	}
	if generationID.Valid {
		message.GenerationID = new(generationID.String)
	}
	var err error
	message.CreatedAt, err = parseTimestamp(createdAt)
	if err != nil {
		return domain.Message{}, err
	}
	return message, nil
}

func scanGenerationVersion(row rowScanner) (domain.GenerationVersion, error) {
	var version domain.GenerationVersion
	var parentVersionID sql.NullString
	var planJSON, specJSON, manifestJSON, checksum, createdAt string
	if err := row.Scan(
		&version.ID,
		&version.ProjectID,
		&parentVersionID,
		&version.Sequence,
		&version.Request,
		&planJSON,
		&specJSON,
		&version.Artifact.EntryHTML,
		&version.Artifact.HTML,
		&version.Artifact.CSS,
		&version.Artifact.JS,
		&manifestJSON,
		&checksum,
		&createdAt,
	); err != nil {
		return domain.GenerationVersion{}, err
	}
	if parentVersionID.Valid {
		version.ParentVersionID = new(parentVersionID.String)
	}
	if err := json.Unmarshal([]byte(planJSON), &version.Plan); err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("decode version plan: %w", err)
	}
	if err := json.Unmarshal([]byte(specJSON), &version.Spec); err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("decode version spec: %w", err)
	}
	if err := version.Plan.NormalizeAndValidate(version.Spec.TemplateName()); err != nil {
		return domain.GenerationVersion{}, err
	}
	if err := json.Unmarshal([]byte(manifestJSON), &version.Artifact.Manifest); err != nil {
		return domain.GenerationVersion{}, fmt.Errorf("decode artifact manifest: %w", err)
	}
	version.Artifact.Manifest.Checksum = strings.TrimSpace(version.Artifact.Manifest.Checksum)
	if version.Artifact.Manifest.Checksum == "" || version.Artifact.Manifest.Checksum != checksum {
		return domain.GenerationVersion{}, errors.New("stored artifact checksum is invalid")
	}
	var err error
	version.CreatedAt, err = parseTimestamp(createdAt)
	if err != nil {
		return domain.GenerationVersion{}, err
	}
	return version, nil
}

func validateArtifact(artifact domain.CompiledArtifact, template domain.AppTemplate) error {
	if artifact.EntryHTML == "" || artifact.HTML == "" || artifact.CSS == "" || artifact.JS == "" {
		return errors.New("compiled artifact is incomplete")
	}
	if artifact.Manifest.Template != template || artifact.Manifest.Checksum == "" {
		return errors.New("compiled artifact manifest is invalid")
	}
	return nil
}
