package compiler

import (
	"strings"
	"testing"

	"github.com/zand/atoms-demo/internal/domain"
)

func testTodoSpec() domain.AppSpec {
	return domain.AppSpec{Todo: &domain.TodoSpec{
		SchemaVersion: 1,
		Template:      domain.TemplateTodo,
		Title:         "安全的 </script><img src=x> 待办",
		Description:   "一个真实可交互的清单。",
		Theme:         domain.TodoThemeOcean,
		Categories:    []string{"工作", "生活"},
		InitialItems: []domain.TodoSeedItem{
			{Text: "完成演示", Category: "工作", Priority: domain.TodoPriorityHigh},
		},
	}}
}

func TestCompileTodoProducesSandboxSafeInteractiveArtifact(t *testing.T) {
	artifact, err := Compile(testTodoSpec())
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Manifest.Template != domain.TemplateTodo || artifact.Manifest.Checksum == "" {
		t.Fatalf("manifest = %#v", artifact.Manifest)
	}
	if !strings.Contains(artifact.EntryHTML, "Content-Security-Policy") || !strings.Contains(artifact.EntryHTML, "connect-src 'none'") {
		t.Fatal("entry artifact does not contain the required CSP")
	}
	if strings.Contains(artifact.EntryHTML, "<script src=") || strings.Contains(artifact.EntryHTML, "</script><img src=x>") {
		t.Fatalf("entry artifact contains unsafe source interpolation: %s", artifact.EntryHTML)
	}
	if !strings.Contains(artifact.JS, "taskForm.addEventListener") || !strings.Contains(artifact.JS, "state.items.splice") {
		t.Fatal("compiled application is missing real todo interactions")
	}
}

func TestCompileTodoChecksumIsStable(t *testing.T) {
	first, err := Compile(testTodoSpec())
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(testTodoSpec())
	if err != nil {
		t.Fatal(err)
	}
	if first.Manifest.Checksum != second.Manifest.Checksum {
		t.Fatalf("checksums differ: %q != %q", first.Manifest.Checksum, second.Manifest.Checksum)
	}
}

func TestCompileNotesAndHabitsProduceInteractiveArtifacts(t *testing.T) {
	notes := domain.AppSpec{Notes: &domain.NotesSpec{
		SchemaVersion: 1,
		Template:      domain.TemplateNotes,
		Title:         "灵感笔记",
		Description:   "记录想法。",
		Theme:         domain.TodoThemeOcean,
		Tags:          []string{"工作"},
		InitialNotes:  []domain.NoteSeedItem{{Title: "方向", Content: "完成原型", Tags: []string{"工作"}}},
	}}
	habits := domain.AppSpec{Habits: &domain.HabitsSpec{
		SchemaVersion: 1,
		Template:      domain.TemplateHabits,
		Title:         "每日习惯",
		Description:   "从小事开始。",
		Theme:         domain.TodoThemeForest,
		InitialHabits: []domain.HabitSeedItem{{Name: "阅读", Icon: domain.HabitIconBook, TargetPerWeek: 5}},
	}}
	for _, spec := range []domain.AppSpec{notes, habits} {
		artifact, err := Compile(spec)
		if err != nil {
			t.Fatal(err)
		}
		if artifact.Manifest.Template != spec.TemplateName() || !strings.Contains(artifact.JS, "atoms-preview:state-change") {
			t.Fatalf("artifact = %#v", artifact.Manifest)
		}
	}
}
