package domain

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	maxDisplayNameLength = 80
	maxProjectNameLength = 100
)

var (
	ErrInvalidDisplayName = errors.New("invalid display name")
	ErrInvalidProjectName = errors.New("invalid project name")
)

type Workspace struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"displayName"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Project struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"-"`
	Name            string    `json:"name"`
	Summary         string    `json:"summary"`
	ActiveVersionID *string   `json:"activeVersionId,omitzero"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func NormalizeDisplayName(value string) (string, error) {
	return normalizeRequired(value, maxDisplayNameLength, ErrInvalidDisplayName)
}

func NormalizeProjectName(value string) (string, error) {
	return normalizeRequired(value, maxProjectNameLength, ErrInvalidProjectName)
}

func normalizeRequired(value string, maximumLength int, invalidError error) (string, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" || utf8.RuneCountInString(normalized) > maximumLength {
		return "", invalidError
	}
	return normalized, nil
}
