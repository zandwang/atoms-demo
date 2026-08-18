package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/zand/atoms-demo/internal/domain"
)

var (
	ErrSessionNotFound           = errors.New("session not found")
	ErrProjectNotFound           = errors.New("project not found")
	ErrGenerationAttemptNotFound = errors.New("generation attempt not found")
)

// Repository is the application-facing persistence boundary.
type Repository interface {
	Close() error
	InitializeWorkspace(ctx context.Context, displayName string) (domain.Workspace, string, error)
	WorkspaceBySession(ctx context.Context, token string) (domain.Workspace, error)
	ListProjects(ctx context.Context, workspaceID string) ([]domain.Project, error)
	ProjectByID(ctx context.Context, workspaceID, projectID string) (domain.Project, error)
	CreateProject(ctx context.Context, workspaceID, name string) (domain.Project, error)
	RenameProject(ctx context.Context, workspaceID, projectID, name string) (domain.Project, error)
	DeleteProject(ctx context.Context, workspaceID, projectID string) error
	ListMessages(ctx context.Context, workspaceID, projectID string, limit int) ([]domain.Message, error)
	ListVersions(ctx context.Context, workspaceID, projectID string) ([]domain.GenerationVersion, error)
	ActiveVersion(ctx context.Context, workspaceID, projectID string) (*domain.GenerationVersion, error)
	BeginGeneration(ctx context.Context, workspaceID, projectID, request string) (domain.GenerationAttempt, error)
	CompleteGeneration(ctx context.Context, workspaceID, projectID, attemptID string, expectedActiveVersionID *string, result domain.AgentResult, artifact domain.CompiledArtifact) (domain.GenerationVersion, error)
	FailGeneration(ctx context.Context, workspaceID, projectID, attemptID, errorCode, message string) error
	ActivateVersion(ctx context.Context, workspaceID, projectID, versionID string) (domain.Project, error)
	PreviewState(ctx context.Context, workspaceID, projectID, versionID string) (json.RawMessage, error)
	SavePreviewState(ctx context.Context, workspaceID, projectID, versionID string, state json.RawMessage) error
}
