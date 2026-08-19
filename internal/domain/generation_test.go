package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func validGeneratedFiles() GeneratedFiles {
	return GeneratedFiles{
		HTML: `<main><h1>番茄钟</h1><output id="timer">25:00</output><button id="start">开始</button></main>`,
		CSS:  `body { font-family: sans-serif; } output { display: block; font-size: 3rem; }`,
		JS:   `const timer = document.getElementById("timer"); document.getElementById("start").addEventListener("click", () => { timer.textContent = "24:59"; window.atomsPreview.publish({remaining: 1499}); });`,
	}
}

func validTodoSpec() TodoSpec {
	return TodoSpec{
		SchemaVersion: appSpecSchemaVersion,
		Template:      TemplateTodo,
		Title:         "今天的待办",
		Description:   "把重要的小事做好。",
		Theme:         TodoThemeViolet,
		Categories:    []string{"工作", "生活"},
		InitialItems: []TodoSeedItem{
			{Text: "整理计划", Category: "工作", Priority: TodoPriorityHigh},
		},
	}
}

func validAgentResult() AgentResult {
	files := validGeneratedFiles()
	return AgentResult{
		Plan: AgentPlan{
			Summary: "创建一个可操作的番茄钟。",
			Steps:   []string{"构建计时器界面", "加入开始与状态保存逻辑"},
		},
		AssistantMessage: "已生成番茄钟，可以继续调整时长和样式。",
		Spec:             AppSpec{Files: &files},
	}
}

func TestParseAgentResultAcceptsStrictGeneratedFiles(t *testing.T) {
	want := validAgentResult()
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}

	got, err := ParseAgentResult(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.Spec.TemplateName() != TemplateCustom || got.Spec.Files.HTML != want.Spec.Files.HTML {
		t.Fatalf("parsed result = %#v", got)
	}
}

func TestParseGeneratedFilesRejectsUnknownUnsafeOversizedAndMixedSpecs(t *testing.T) {
	valid := `{"schemaVersion":1,"template":"custom","files":{"indexHtml":"<main>Timer</main>","stylesCss":"body {}","appJs":"const ready = true;"}}`
	if _, err := ParseAppSpec([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{
		`{"schemaVersion":1,"template":"custom","files":{"indexHtml":"<main>Timer</main>","stylesCss":"","appJs":"","extra":true}}`,
		`{"schemaVersion":1,"template":"custom","files":{"indexHtml":"<script>alert(1)</script>","stylesCss":"","appJs":""}}`,
		`{"schemaVersion":1,"template":"custom","files":{"indexHtml":"<main>Timer</main>","stylesCss":"","appJs":"fetch('/api')"}}`,
	} {
		if _, err := ParseAppSpec([]byte(payload)); !errors.Is(err, ErrInvalidAppSpec) {
			t.Fatalf("ParseAppSpec(%s) error = %v, want ErrInvalidAppSpec", payload, err)
		}
	}

	oversized := GeneratedFiles{HTML: `<main>` + strings.Repeat("x", maxGeneratedHTML) + `</main>`}
	if err := oversized.NormalizeAndValidate(); !errors.Is(err, ErrInvalidAppSpec) {
		t.Fatalf("oversized source error = %v, want ErrInvalidAppSpec", err)
	}
	todo := validTodoSpec()
	files := validGeneratedFiles()
	mixed := AppSpec{Files: &files, Todo: &todo}
	if err := mixed.NormalizeAndValidate(); !errors.Is(err, ErrInvalidAppSpec) {
		t.Fatalf("mixed spec error = %v, want ErrInvalidAppSpec", err)
	}
	if _, err := json.Marshal(mixed); !errors.Is(err, ErrInvalidAppSpec) {
		t.Fatalf("Marshal(mixed) error = %v, want ErrInvalidAppSpec", err)
	}
}

func TestParseAppSpecRejectsUnknownFieldsAndUnknownCategories(t *testing.T) {
	todo := validTodoSpec()
	encoded, err := json.Marshal(todo)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	payload["unexpected"] = true
	withUnknownField, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAppSpec(withUnknownField); !errors.Is(err, ErrInvalidAppSpec) {
		t.Fatalf("ParseAppSpec() error = %v, want ErrInvalidAppSpec", err)
	}

	payload = map[string]any{}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	items := payload["initialItems"].([]any)
	items[0].(map[string]any)["category"] = "不存在"
	invalidCategory, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseAppSpec(invalidCategory); !errors.Is(err, ErrInvalidAppSpec) {
		t.Fatalf("ParseAppSpec() error = %v, want ErrInvalidAppSpec", err)
	}
}

func TestParseAppSpecSupportsNotesAndHabits(t *testing.T) {
	notes, err := ParseAppSpec([]byte(`{"schemaVersion":1,"template":"notes","title":"灵感笔记","description":"记录想法","theme":"ocean","tags":["工作"],"initialNotes":[{"title":"方向","content":"先完成原型","tags":["工作"]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if notes.TemplateName() != TemplateNotes || notes.Notes == nil {
		t.Fatalf("notes = %#v", notes)
	}
	habits, err := ParseAppSpec([]byte(`{"schemaVersion":1,"template":"habits","title":"每日习惯","description":"从小事开始","theme":"forest","initialHabits":[{"name":"阅读","icon":"book","targetPerWeek":5}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if habits.TemplateName() != TemplateHabits || habits.Habits == nil {
		t.Fatalf("habits = %#v", habits)
	}
	if _, err := ParseAppSpec([]byte(`{"schemaVersion":1,"template":"unknown"}`)); !errors.Is(err, ErrUnsupportedTemplate) {
		t.Fatalf("ParseAppSpec() error = %v, want ErrUnsupportedTemplate", err)
	}
}

func TestNormalizeGenerationRequest(t *testing.T) {
	request, err := NormalizeGenerationRequest("  做一个待办应用  ")
	if err != nil {
		t.Fatal(err)
	}
	if request != "做一个待办应用" {
		t.Fatalf("request = %q", request)
	}
	if _, err := NormalizeGenerationRequest(""); !errors.Is(err, ErrInvalidGenerationRequest) {
		t.Fatalf("NormalizeGenerationRequest() error = %v", err)
	}
}

func TestNormalizePreviewStateValidatesTemplateState(t *testing.T) {
	files := validGeneratedFiles()
	customState, err := NormalizePreviewState(AppSpec{Files: &files}, json.RawMessage(`{"remaining":1499,"running":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(customState), `"remaining":1499`) {
		t.Fatalf("custom state = %s", customState)
	}
	if _, err := NormalizePreviewState(AppSpec{Files: &files}, json.RawMessage(`[1,2,3]`)); !errors.Is(err, ErrInvalidPreviewState) {
		t.Fatalf("array custom state error = %v, want ErrInvalidPreviewState", err)
	}

	todo := validTodoSpec()
	state, err := NormalizePreviewState(AppSpec{Todo: &todo}, json.RawMessage(`{"items":[{"id":"one","text":"整理计划","category":"工作","priority":"high","done":true}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(state) == "" {
		t.Fatal("normalized state is empty")
	}
	_, err = NormalizePreviewState(AppSpec{Todo: &todo}, json.RawMessage(`{"items":[{"id":"one","text":"整理计划","category":"未知","priority":"high","done":true}]}`))
	if !errors.Is(err, ErrInvalidPreviewState) {
		t.Fatalf("NormalizePreviewState() error = %v, want ErrInvalidPreviewState", err)
	}
	notes := NotesSpec{
		SchemaVersion: 1,
		Template:      TemplateNotes,
		Title:         "笔记",
		Description:   "记录",
		Theme:         TodoThemeOcean,
		Tags:          []string{"工作"},
	}
	if _, err := NormalizePreviewState(AppSpec{Notes: &notes}, json.RawMessage(`{"notes":[{"id":"n1","title":"方向","content":"原型","tags":["工作"]}]}`)); err != nil {
		t.Fatal(err)
	}
	habits := HabitsSpec{
		SchemaVersion: 1,
		Template:      TemplateHabits,
		Title:         "习惯",
		Description:   "打卡",
		Theme:         TodoThemeForest,
	}
	if _, err := NormalizePreviewState(AppSpec{Habits: &habits}, json.RawMessage(`{"habits":[{"id":"h1","name":"阅读","icon":"book","targetPerWeek":5,"completedDates":["2026-08-18"]}]}`)); err != nil {
		t.Fatal(err)
	}
}
