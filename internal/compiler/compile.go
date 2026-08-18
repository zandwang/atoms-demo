// Package compiler turns validated, constrained application specifications into
// standalone preview artifacts. It never accepts model-provided source code.
package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/zand/atoms-demo/internal/domain"
)

// Compile produces a deterministic artifact for a supported application spec.
func Compile(spec domain.AppSpec) (domain.CompiledArtifact, error) {
	if err := spec.NormalizeAndValidate(); err != nil {
		return domain.CompiledArtifact{}, err
	}

	switch spec.TemplateName() {
	case domain.TemplateTodo:
		return CompileTodo(*spec.Todo)
	case domain.TemplateNotes:
		return CompileNotes(*spec.Notes)
	case domain.TemplateHabits:
		return CompileHabits(*spec.Habits)
	default:
		return domain.CompiledArtifact{}, fmt.Errorf("%w: %q", domain.ErrUnsupportedTemplate, spec.TemplateName())
	}
}

func artifactChecksum(entryHTML string) string {
	sum := sha256.Sum256([]byte(entryHTML))
	return hex.EncodeToString(sum[:])
}
