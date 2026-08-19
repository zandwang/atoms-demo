// Package compiler turns validated, constrained application files into
// standalone sandbox preview artifacts.
package compiler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/zand/atoms-demo/internal/domain"
)

// Compile produces a deterministic artifact for generated files or a legacy spec.
func Compile(spec domain.AppSpec) (domain.CompiledArtifact, error) {
	if err := spec.NormalizeAndValidate(); err != nil {
		return domain.CompiledArtifact{}, err
	}

	switch spec.TemplateName() {
	case domain.TemplateCustom:
		return CompileGenerated(*spec.Files)
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
