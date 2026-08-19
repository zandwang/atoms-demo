package sqlite

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zand/atoms-demo/internal/compiler"
	"github.com/zand/atoms-demo/internal/domain"
	"github.com/zand/atoms-demo/internal/store"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	persistence, err := OpenPath(filepath.Join(t.TempDir(), "atoms-demo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := persistence.Close(); err != nil {
			t.Error(err)
		}
	})
	return persistence
}

func TestWorkspaceSessionAndProjectLifecycle(t *testing.T) {
	persistence := newTestStore(t)
	ctx := t.Context()

	workspace, token, err := persistence.InitializeWorkspace(ctx, "Zand")
	if err != nil {
		t.Fatal(err)
	}
	foundWorkspace, err := persistence.WorkspaceBySession(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if foundWorkspace.ID != workspace.ID || foundWorkspace.DisplayName != "Zand" {
		t.Fatalf("workspace = %#v", foundWorkspace)
	}

	project, err := persistence.CreateProject(ctx, workspace.ID, "My first app")
	if err != nil {
		t.Fatal(err)
	}
	projects, err := persistence.ListProjects(ctx, workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].ID != project.ID {
		t.Fatalf("projects = %#v", projects)
	}

	renamed, err := persistence.RenameProject(ctx, workspace.ID, project.ID, "Renamed app")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "Renamed app" {
		t.Fatalf("renamed project = %#v", renamed)
	}

	if err := persistence.DeleteProject(ctx, workspace.ID, project.ID); err != nil {
		t.Fatal(err)
	}
	projects, err = persistence.ListProjects(ctx, workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 0 {
		t.Fatalf("projects after deletion = %#v", projects)
	}
}

func TestProjectIsScopedToWorkspace(t *testing.T) {
	persistence := newTestStore(t)
	ctx := t.Context()
	firstWorkspace, _, err := persistence.InitializeWorkspace(ctx, "First")
	if err != nil {
		t.Fatal(err)
	}
	secondWorkspace, _, err := persistence.InitializeWorkspace(ctx, "Second")
	if err != nil {
		t.Fatal(err)
	}
	project, err := persistence.CreateProject(ctx, firstWorkspace.ID, "Private")
	if err != nil {
		t.Fatal(err)
	}

	if err := persistence.DeleteProject(ctx, secondWorkspace.ID, project.ID); !errors.Is(err, store.ErrProjectNotFound) {
		t.Fatalf("DeleteProject() error = %v, want ErrProjectNotFound", err)
	}
}

func TestGenerationLifecyclePersistsMessagesVersionAndActiveArtifact(t *testing.T) {
	persistence := newTestStore(t)
	ctx := t.Context()
	workspace, _, err := persistence.InitializeWorkspace(ctx, "Zand")
	if err != nil {
		t.Fatal(err)
	}
	project, err := persistence.CreateProject(ctx, workspace.ID, "Generated app")
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := persistence.BeginGeneration(ctx, workspace.ID, project.ID, "做一个番茄钟")
	if err != nil {
		t.Fatal(err)
	}
	result := testAgentResult()
	artifact, err := compiler.Compile(result.Spec)
	if err != nil {
		t.Fatal(err)
	}
	version, err := persistence.CompleteGeneration(ctx, workspace.ID, project.ID, attempt.ID, nil, result, artifact)
	if err != nil {
		t.Fatal(err)
	}
	if version.Sequence != 1 || version.Artifact.Manifest.Checksum == "" {
		t.Fatalf("version = %#v", version)
	}

	updatedProject, err := persistence.ProjectByID(ctx, workspace.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedProject.ActiveVersionID == nil || *updatedProject.ActiveVersionID != version.ID {
		t.Fatalf("active version = %#v, want %q", updatedProject.ActiveVersionID, version.ID)
	}
	messages, err := persistence.ListMessages(ctx, workspace.ID, project.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].Role != domain.MessageRoleUser || messages[1].Role != domain.MessageRoleAssistant {
		t.Fatalf("messages = %#v", messages)
	}
	versions, err := persistence.ListVersions(ctx, workspace.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0].ID != version.ID || versions[0].Spec.Files == nil || !strings.Contains(versions[0].Spec.Files.HTML, "番茄钟") {
		t.Fatalf("versions = %#v", versions)
	}
	active, err := persistence.ActiveVersion(ctx, workspace.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active == nil || active.ID != version.ID {
		t.Fatalf("active version = %#v", active)
	}

	state := json.RawMessage(`{"remaining":1499,"running":true}`)
	if err := persistence.SavePreviewState(ctx, workspace.ID, project.ID, version.ID, state); err != nil {
		t.Fatal(err)
	}
	restoredState, err := persistence.PreviewState(ctx, workspace.ID, project.ID, version.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(restoredState) != string(state) {
		t.Fatalf("preview state = %s, want %s", restoredState, state)
	}
	invalidState := json.RawMessage(`["state must be an object"]`)
	if err := persistence.SavePreviewState(ctx, workspace.ID, project.ID, version.ID, invalidState); !errors.Is(err, domain.ErrInvalidPreviewState) {
		t.Fatalf("SavePreviewState() error = %v, want ErrInvalidPreviewState", err)
	}

	secondAttempt, err := persistence.BeginGeneration(ctx, workspace.ID, project.ID, "改成 50 分钟专注")
	if err != nil {
		t.Fatal(err)
	}
	secondResult := testAgentResult()
	secondResult.Plan.Summary = "把番茄钟改成 50 分钟。"
	secondResult.Spec.Files.HTML = strings.Replace(secondResult.Spec.Files.HTML, "25:00", "50:00", 1)
	secondArtifact, err := compiler.Compile(secondResult.Spec)
	if err != nil {
		t.Fatal(err)
	}
	secondVersion, err := persistence.CompleteGeneration(ctx, workspace.ID, project.ID, secondAttempt.ID, &version.ID, secondResult, secondArtifact)
	if err != nil {
		t.Fatal(err)
	}
	if secondVersion.ParentVersionID == nil || *secondVersion.ParentVersionID != version.ID {
		t.Fatalf("parent version = %#v, want %q", secondVersion.ParentVersionID, version.ID)
	}
	thirdAttempt, err := persistence.BeginGeneration(ctx, workspace.ID, project.ID, "生成一个不会覆盖回滚的版本")
	if err != nil {
		t.Fatal(err)
	}
	activatedProject, err := persistence.ActivateVersion(ctx, workspace.ID, project.ID, version.ID)
	if err != nil {
		t.Fatal(err)
	}
	if activatedProject.ActiveVersionID == nil || *activatedProject.ActiveVersionID != version.ID {
		t.Fatalf("activated project = %#v", activatedProject)
	}
	thirdResult := testAgentResult()
	thirdResult.Plan.Summary = "不会覆盖当前版本的新结果。"
	thirdResult.Spec.Files.HTML = strings.Replace(thirdResult.Spec.Files.HTML, "25:00", "45:00", 1)
	thirdArtifact, err := compiler.Compile(thirdResult.Spec)
	if err != nil {
		t.Fatal(err)
	}
	thirdVersion, err := persistence.CompleteGeneration(ctx, workspace.ID, project.ID, thirdAttempt.ID, &secondVersion.ID, thirdResult, thirdArtifact)
	if err != nil {
		t.Fatal(err)
	}
	if thirdVersion.ParentVersionID == nil || *thirdVersion.ParentVersionID != secondVersion.ID {
		t.Fatalf("third version parent = %#v, want %q", thirdVersion.ParentVersionID, secondVersion.ID)
	}
	projectAfterLateResult, err := persistence.ProjectByID(ctx, workspace.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if projectAfterLateResult.ActiveVersionID == nil || *projectAfterLateResult.ActiveVersionID != version.ID {
		t.Fatalf("late result overwrote restored active version: %#v", projectAfterLateResult.ActiveVersionID)
	}
}

func TestFailedGenerationKeepsUserRequestAndAddsSystemMessage(t *testing.T) {
	persistence := newTestStore(t)
	ctx := t.Context()
	workspace, _, err := persistence.InitializeWorkspace(ctx, "Zand")
	if err != nil {
		t.Fatal(err)
	}
	project, err := persistence.CreateProject(ctx, workspace.ID, "Failed app")
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := persistence.BeginGeneration(ctx, workspace.ID, project.ID, "做一个待办")
	if err != nil {
		t.Fatal(err)
	}
	if err := persistence.FailGeneration(ctx, workspace.ID, project.ID, attempt.ID, "UPSTREAM_TIMEOUT", "模型响应超时，请重试。"); err != nil {
		t.Fatal(err)
	}
	messages, err := persistence.ListMessages(ctx, workspace.ID, project.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[1].Role != domain.MessageRoleSystem || messages[1].Content != "模型响应超时，请重试。" {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestVersionAndPreviewStateSurviveStoreReopen(t *testing.T) {
	ctx := t.Context()
	databasePath := filepath.Join(t.TempDir(), "persistent-atoms-demo.db")
	firstStore, err := OpenPath(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	workspace, token, err := firstStore.InitializeWorkspace(ctx, "Persistent user")
	if err != nil {
		t.Fatal(err)
	}
	project, err := firstStore.CreateProject(ctx, workspace.ID, "Persistent project")
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := firstStore.BeginGeneration(ctx, workspace.ID, project.ID, "做一个番茄钟")
	if err != nil {
		t.Fatal(err)
	}
	result := testAgentResult()
	artifact, err := compiler.Compile(result.Spec)
	if err != nil {
		t.Fatal(err)
	}
	version, err := firstStore.CompleteGeneration(ctx, workspace.ID, project.ID, attempt.ID, nil, result, artifact)
	if err != nil {
		t.Fatal(err)
	}
	state := json.RawMessage(`{"remaining":1234,"running":false}`)
	if err := firstStore.SavePreviewState(ctx, workspace.ID, project.ID, version.ID, state); err != nil {
		t.Fatal(err)
	}
	if err := firstStore.Close(); err != nil {
		t.Fatal(err)
	}

	secondStore, err := OpenPath(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := secondStore.Close(); err != nil {
			t.Error(err)
		}
	})
	recoveredWorkspace, err := secondStore.WorkspaceBySession(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if recoveredWorkspace.ID != workspace.ID {
		t.Fatalf("workspace after reopen = %#v", recoveredWorkspace)
	}
	recoveredProject, err := secondStore.ProjectByID(ctx, workspace.ID, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recoveredProject.ActiveVersionID == nil || *recoveredProject.ActiveVersionID != version.ID {
		t.Fatalf("project after reopen = %#v", recoveredProject)
	}
	recoveredState, err := secondStore.PreviewState(ctx, workspace.ID, project.ID, version.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(recoveredState) != string(state) {
		t.Fatalf("state after reopen = %s, want %s", recoveredState, state)
	}
}

func testAgentResult() domain.AgentResult {
	files := domain.GeneratedFiles{
		HTML: `<main><h1>番茄钟</h1><output id="timer">25:00</output><button id="start">开始</button></main>`,
		CSS:  `body { font-family: sans-serif; } output { display: block; font-size: 3rem; }`,
		JS:   `const timer = document.getElementById("timer"); document.getElementById("start").addEventListener("click", () => { timer.textContent = "24:59"; window.atomsPreview.publish({remaining: 1499, running: true}); });`,
	}
	return domain.AgentResult{
		Plan: domain.AgentPlan{
			Summary: "创建一个可操作的番茄钟。",
			Steps:   []string{"创建计时界面", "添加开始和状态保存逻辑"},
		},
		AssistantMessage: "已生成番茄钟。",
		Spec:             domain.AppSpec{Files: &files},
	}
}
