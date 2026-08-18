package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	appSpecSchemaVersion  = 1
	maxGenerationRequest  = 2_000
	maxPlanSummary        = 240
	maxPlanSteps          = 4
	maxAssistantMessage   = 500
	maxTodoTitle          = 80
	maxTodoDescription    = 240
	maxTodoCategories     = 8
	maxTodoCategoryLength = 32
	maxTodoItems          = 12
	maxTodoItemText       = 160
	maxNotesTags          = 8
	maxNotesTagLength     = 32
	maxNotesItems         = 12
	maxNoteTitle          = 100
	maxNoteContent        = 800
	maxHabitItems         = 12
	maxHabitName          = 80
)

var (
	ErrInvalidGenerationRequest = errors.New("invalid generation request")
	ErrInvalidAppSpec           = errors.New("invalid app spec")
	ErrUnsupportedTemplate      = errors.New("unsupported app template")
	ErrInvalidAgentResult       = errors.New("invalid agent result")
	ErrInvalidPreviewState      = errors.New("invalid preview state")
)

type AppTemplate string

const (
	TemplateTodo   AppTemplate = "todo"
	TemplateNotes  AppTemplate = "notes"
	TemplateHabits AppTemplate = "habits"
)

type TodoTheme string

const (
	TodoThemeViolet TodoTheme = "violet"
	TodoThemeOcean  TodoTheme = "ocean"
	TodoThemeForest TodoTheme = "forest"
	TodoThemeSunset TodoTheme = "sunset"
	TodoThemeSlate  TodoTheme = "slate"
)

type TodoPriority string

const (
	TodoPriorityLow    TodoPriority = "low"
	TodoPriorityMedium TodoPriority = "medium"
	TodoPriorityHigh   TodoPriority = "high"
)

// TodoSpec is the only executable application schema in M3. It deliberately
// contains data and enum values only; no arbitrary HTML, CSS, or JavaScript.
type TodoSpec struct {
	SchemaVersion int            `json:"schemaVersion"`
	Template      AppTemplate    `json:"template"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	Theme         TodoTheme      `json:"theme"`
	Categories    []string       `json:"categories"`
	InitialItems  []TodoSeedItem `json:"initialItems"`
}

type TodoSeedItem struct {
	Text     string       `json:"text"`
	Category string       `json:"category"`
	Priority TodoPriority `json:"priority"`
}

type NotesSpec struct {
	SchemaVersion int            `json:"schemaVersion"`
	Template      AppTemplate    `json:"template"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	Theme         TodoTheme      `json:"theme"`
	Tags          []string       `json:"tags"`
	InitialNotes  []NoteSeedItem `json:"initialNotes"`
}

type NoteSeedItem struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

type HabitIcon string

const (
	HabitIconCheck HabitIcon = "check"
	HabitIconHeart HabitIcon = "heart"
	HabitIconBook  HabitIcon = "book"
	HabitIconRun   HabitIcon = "run"
	HabitIconWater HabitIcon = "water"
)

type HabitsSpec struct {
	SchemaVersion int             `json:"schemaVersion"`
	Template      AppTemplate     `json:"template"`
	Title         string          `json:"title"`
	Description   string          `json:"description"`
	Theme         TodoTheme       `json:"theme"`
	InitialHabits []HabitSeedItem `json:"initialHabits"`
}

type HabitSeedItem struct {
	Name          string    `json:"name"`
	Icon          HabitIcon `json:"icon"`
	TargetPerWeek int       `json:"targetPerWeek"`
}

// AppSpec is a discriminated union. Its custom JSON codec keeps the external
// representation flat while preventing callers from supplying arbitrary maps.
type AppSpec struct {
	Todo   *TodoSpec
	Notes  *NotesSpec
	Habits *HabitsSpec
}

func (s AppSpec) TemplateName() AppTemplate {
	switch {
	case s.Todo != nil:
		return s.Todo.Template
	case s.Notes != nil:
		return s.Notes.Template
	case s.Habits != nil:
		return s.Habits.Template
	default:
		return ""
	}
}

func (s AppSpec) MarshalJSON() ([]byte, error) {
	switch {
	case s.Todo != nil && s.Notes == nil && s.Habits == nil:
		copy := *s.Todo
		if err := copy.NormalizeAndValidate(); err != nil {
			return nil, err
		}
		return json.Marshal(copy)
	case s.Notes != nil && s.Todo == nil && s.Habits == nil:
		copy := *s.Notes
		if err := copy.NormalizeAndValidate(); err != nil {
			return nil, err
		}
		return json.Marshal(copy)
	case s.Habits != nil && s.Todo == nil && s.Notes == nil:
		copy := *s.Habits
		if err := copy.NormalizeAndValidate(); err != nil {
			return nil, err
		}
		return json.Marshal(copy)
	default:
		return nil, fmt.Errorf("%w: exactly one application specification is required", ErrInvalidAppSpec)
	}
}

func (s *AppSpec) UnmarshalJSON(data []byte) error {
	parsed, err := ParseAppSpec(data)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
}

func (s *AppSpec) NormalizeAndValidate() error {
	if s == nil {
		return fmt.Errorf("%w: missing application specification", ErrInvalidAppSpec)
	}
	switch {
	case s.Todo != nil && s.Notes == nil && s.Habits == nil:
		return s.Todo.NormalizeAndValidate()
	case s.Notes != nil && s.Todo == nil && s.Habits == nil:
		return s.Notes.NormalizeAndValidate()
	case s.Habits != nil && s.Todo == nil && s.Notes == nil:
		return s.Habits.NormalizeAndValidate()
	default:
		return fmt.Errorf("%w: exactly one application specification is required", ErrInvalidAppSpec)
	}
}

func ParseAppSpec(data []byte) (AppSpec, error) {
	var discriminant struct {
		Template AppTemplate `json:"template"`
	}
	if err := json.Unmarshal(data, &discriminant); err != nil {
		return AppSpec{}, fmt.Errorf("%w: decode template discriminator: %v", ErrInvalidAppSpec, err)
	}

	switch discriminant.Template {
	case TemplateTodo:
		var todo TodoSpec
		if err := decodeStrictJSON(data, &todo); err != nil {
			return AppSpec{}, fmt.Errorf("%w: decode todo specification: %v", ErrInvalidAppSpec, err)
		}
		if err := todo.NormalizeAndValidate(); err != nil {
			return AppSpec{}, err
		}
		return AppSpec{Todo: &todo}, nil
	case TemplateNotes:
		var notes NotesSpec
		if err := decodeStrictJSON(data, &notes); err != nil {
			return AppSpec{}, fmt.Errorf("%w: decode notes specification: %v", ErrInvalidAppSpec, err)
		}
		if err := notes.NormalizeAndValidate(); err != nil {
			return AppSpec{}, err
		}
		return AppSpec{Notes: &notes}, nil
	case TemplateHabits:
		var habits HabitsSpec
		if err := decodeStrictJSON(data, &habits); err != nil {
			return AppSpec{}, fmt.Errorf("%w: decode habits specification: %v", ErrInvalidAppSpec, err)
		}
		if err := habits.NormalizeAndValidate(); err != nil {
			return AppSpec{}, err
		}
		return AppSpec{Habits: &habits}, nil
	default:
		return AppSpec{}, fmt.Errorf("%w: unknown template %q", ErrUnsupportedTemplate, discriminant.Template)
	}
}

func (s *TodoSpec) NormalizeAndValidate() error {
	if s == nil {
		return fmt.Errorf("%w: missing todo specification", ErrInvalidAppSpec)
	}
	if s.SchemaVersion != appSpecSchemaVersion {
		return fmt.Errorf("%w: schemaVersion must be %d", ErrInvalidAppSpec, appSpecSchemaVersion)
	}
	if s.Template != TemplateTodo {
		return fmt.Errorf("%w: todo specification has template %q", ErrInvalidAppSpec, s.Template)
	}

	var err error
	if s.Title, err = normalizeAppText(s.Title, maxTodoTitle, true); err != nil {
		return fmt.Errorf("%w: title %v", ErrInvalidAppSpec, err)
	}
	if s.Description, err = normalizeAppText(s.Description, maxTodoDescription, false); err != nil {
		return fmt.Errorf("%w: description %v", ErrInvalidAppSpec, err)
	}
	if !isTodoTheme(s.Theme) {
		return fmt.Errorf("%w: unknown theme %q", ErrInvalidAppSpec, s.Theme)
	}
	if len(s.Categories) == 0 || len(s.Categories) > maxTodoCategories {
		return fmt.Errorf("%w: categories must contain 1–%d values", ErrInvalidAppSpec, maxTodoCategories)
	}

	categorySet := make(map[string]struct{}, len(s.Categories))
	for index, category := range s.Categories {
		normalized, err := normalizeAppText(category, maxTodoCategoryLength, true)
		if err != nil {
			return fmt.Errorf("%w: category %d %v", ErrInvalidAppSpec, index+1, err)
		}
		key := strings.ToLower(normalized)
		if _, exists := categorySet[key]; exists {
			return fmt.Errorf("%w: category %q is duplicated", ErrInvalidAppSpec, normalized)
		}
		categorySet[key] = struct{}{}
		s.Categories[index] = normalized
	}

	if len(s.InitialItems) > maxTodoItems {
		return fmt.Errorf("%w: initialItems exceeds %d entries", ErrInvalidAppSpec, maxTodoItems)
	}
	for index := range s.InitialItems {
		item := &s.InitialItems[index]
		item.Text, err = normalizeAppText(item.Text, maxTodoItemText, true)
		if err != nil {
			return fmt.Errorf("%w: initial item %d text %v", ErrInvalidAppSpec, index+1, err)
		}
		item.Category, err = normalizeAppText(item.Category, maxTodoCategoryLength, true)
		if err != nil {
			return fmt.Errorf("%w: initial item %d category %v", ErrInvalidAppSpec, index+1, err)
		}
		if _, exists := categorySet[strings.ToLower(item.Category)]; !exists {
			return fmt.Errorf("%w: initial item %d references unknown category %q", ErrInvalidAppSpec, index+1, item.Category)
		}
		if !isTodoPriority(item.Priority) {
			return fmt.Errorf("%w: initial item %d has unknown priority %q", ErrInvalidAppSpec, index+1, item.Priority)
		}
	}

	return nil
}

func (s *NotesSpec) NormalizeAndValidate() error {
	if s == nil {
		return fmt.Errorf("%w: missing notes specification", ErrInvalidAppSpec)
	}
	if s.SchemaVersion != appSpecSchemaVersion {
		return fmt.Errorf("%w: schemaVersion must be %d", ErrInvalidAppSpec, appSpecSchemaVersion)
	}
	if s.Template != TemplateNotes {
		return fmt.Errorf("%w: notes specification has template %q", ErrInvalidAppSpec, s.Template)
	}
	var err error
	if s.Title, err = normalizeAppText(s.Title, maxTodoTitle, true); err != nil {
		return fmt.Errorf("%w: title %v", ErrInvalidAppSpec, err)
	}
	if s.Description, err = normalizeAppText(s.Description, maxTodoDescription, false); err != nil {
		return fmt.Errorf("%w: description %v", ErrInvalidAppSpec, err)
	}
	if !isTodoTheme(s.Theme) {
		return fmt.Errorf("%w: unknown theme %q", ErrInvalidAppSpec, s.Theme)
	}
	if len(s.Tags) == 0 || len(s.Tags) > maxNotesTags {
		return fmt.Errorf("%w: tags must contain 1–%d values", ErrInvalidAppSpec, maxNotesTags)
	}
	tagSet := make(map[string]struct{}, len(s.Tags))
	for index, tag := range s.Tags {
		normalized, err := normalizeAppText(tag, maxNotesTagLength, true)
		if err != nil {
			return fmt.Errorf("%w: tag %d %v", ErrInvalidAppSpec, index+1, err)
		}
		key := strings.ToLower(normalized)
		if _, exists := tagSet[key]; exists {
			return fmt.Errorf("%w: tag %q is duplicated", ErrInvalidAppSpec, normalized)
		}
		tagSet[key] = struct{}{}
		s.Tags[index] = normalized
	}
	if len(s.InitialNotes) > maxNotesItems {
		return fmt.Errorf("%w: initialNotes exceeds %d entries", ErrInvalidAppSpec, maxNotesItems)
	}
	for index := range s.InitialNotes {
		note := &s.InitialNotes[index]
		note.Title, err = normalizeAppText(note.Title, maxNoteTitle, true)
		if err != nil {
			return fmt.Errorf("%w: initial note %d title %v", ErrInvalidAppSpec, index+1, err)
		}
		note.Content, err = normalizeAppText(note.Content, maxNoteContent, false)
		if err != nil {
			return fmt.Errorf("%w: initial note %d content %v", ErrInvalidAppSpec, index+1, err)
		}
		if len(note.Tags) > 3 {
			return fmt.Errorf("%w: initial note %d has too many tags", ErrInvalidAppSpec, index+1)
		}
		usedTags := make(map[string]struct{}, len(note.Tags))
		for tagIndex, tag := range note.Tags {
			normalized, err := normalizeAppText(tag, maxNotesTagLength, true)
			if err != nil {
				return fmt.Errorf("%w: initial note %d tag %d %v", ErrInvalidAppSpec, index+1, tagIndex+1, err)
			}
			key := strings.ToLower(normalized)
			if _, exists := tagSet[key]; !exists {
				return fmt.Errorf("%w: initial note %d references unknown tag %q", ErrInvalidAppSpec, index+1, normalized)
			}
			if _, exists := usedTags[key]; exists {
				return fmt.Errorf("%w: initial note %d repeats tag %q", ErrInvalidAppSpec, index+1, normalized)
			}
			usedTags[key] = struct{}{}
			note.Tags[tagIndex] = normalized
		}
	}
	return nil
}

func (s *HabitsSpec) NormalizeAndValidate() error {
	if s == nil {
		return fmt.Errorf("%w: missing habits specification", ErrInvalidAppSpec)
	}
	if s.SchemaVersion != appSpecSchemaVersion {
		return fmt.Errorf("%w: schemaVersion must be %d", ErrInvalidAppSpec, appSpecSchemaVersion)
	}
	if s.Template != TemplateHabits {
		return fmt.Errorf("%w: habits specification has template %q", ErrInvalidAppSpec, s.Template)
	}
	var err error
	if s.Title, err = normalizeAppText(s.Title, maxTodoTitle, true); err != nil {
		return fmt.Errorf("%w: title %v", ErrInvalidAppSpec, err)
	}
	if s.Description, err = normalizeAppText(s.Description, maxTodoDescription, false); err != nil {
		return fmt.Errorf("%w: description %v", ErrInvalidAppSpec, err)
	}
	if !isTodoTheme(s.Theme) {
		return fmt.Errorf("%w: unknown theme %q", ErrInvalidAppSpec, s.Theme)
	}
	if len(s.InitialHabits) > maxHabitItems {
		return fmt.Errorf("%w: initialHabits exceeds %d entries", ErrInvalidAppSpec, maxHabitItems)
	}
	for index := range s.InitialHabits {
		habit := &s.InitialHabits[index]
		habit.Name, err = normalizeAppText(habit.Name, maxHabitName, true)
		if err != nil {
			return fmt.Errorf("%w: initial habit %d name %v", ErrInvalidAppSpec, index+1, err)
		}
		if !isHabitIcon(habit.Icon) {
			return fmt.Errorf("%w: initial habit %d has unknown icon %q", ErrInvalidAppSpec, index+1, habit.Icon)
		}
		if habit.TargetPerWeek < 1 || habit.TargetPerWeek > 7 {
			return fmt.Errorf("%w: initial habit %d targetPerWeek must be 1–7", ErrInvalidAppSpec, index+1)
		}
	}
	return nil
}

func isTodoTheme(theme TodoTheme) bool {
	switch theme {
	case TodoThemeViolet, TodoThemeOcean, TodoThemeForest, TodoThemeSunset, TodoThemeSlate:
		return true
	default:
		return false
	}
}

func isTodoPriority(priority TodoPriority) bool {
	switch priority {
	case TodoPriorityLow, TodoPriorityMedium, TodoPriorityHigh:
		return true
	default:
		return false
	}
}

func isHabitIcon(icon HabitIcon) bool {
	switch icon {
	case HabitIconCheck, HabitIconHeart, HabitIconBook, HabitIconRun, HabitIconWater:
		return true
	default:
		return false
	}
}

type AgentPlan struct {
	Summary          string      `json:"summary"`
	Steps            []string    `json:"steps"`
	SelectedTemplate AppTemplate `json:"selectedTemplate"`
}

func (p *AgentPlan) NormalizeAndValidate(template AppTemplate) error {
	if p == nil {
		return fmt.Errorf("%w: missing plan", ErrInvalidAgentResult)
	}
	var err error
	if p.Summary, err = normalizeAppText(p.Summary, maxPlanSummary, true); err != nil {
		return fmt.Errorf("%w: plan summary %v", ErrInvalidAgentResult, err)
	}
	if len(p.Steps) == 0 || len(p.Steps) > maxPlanSteps {
		return fmt.Errorf("%w: plan must contain 1–%d steps", ErrInvalidAgentResult, maxPlanSteps)
	}
	for index, step := range p.Steps {
		normalized, err := normalizeAppText(step, maxPlanSummary, true)
		if err != nil {
			return fmt.Errorf("%w: plan step %d %v", ErrInvalidAgentResult, index+1, err)
		}
		p.Steps[index] = normalized
	}
	if p.SelectedTemplate != template {
		return fmt.Errorf("%w: selectedTemplate %q does not match appSpec", ErrInvalidAgentResult, p.SelectedTemplate)
	}
	return nil
}

type AgentResult struct {
	Plan             AgentPlan `json:"plan"`
	AssistantMessage string    `json:"assistantMessage"`
	Spec             AppSpec   `json:"appSpec"`
}

func (r *AgentResult) NormalizeAndValidate() error {
	if r == nil {
		return fmt.Errorf("%w: missing result", ErrInvalidAgentResult)
	}
	if err := r.Spec.NormalizeAndValidate(); err != nil {
		return err
	}
	if err := r.Plan.NormalizeAndValidate(r.Spec.TemplateName()); err != nil {
		return err
	}
	var err error
	if r.AssistantMessage, err = normalizeAppText(r.AssistantMessage, maxAssistantMessage, true); err != nil {
		return fmt.Errorf("%w: assistant message %v", ErrInvalidAgentResult, err)
	}
	return nil
}

func ParseAgentResult(data []byte) (AgentResult, error) {
	var result AgentResult
	if err := decodeStrictJSON(data, &result); err != nil {
		return AgentResult{}, fmt.Errorf("%w: decode JSON: %v", ErrInvalidAgentResult, err)
	}
	if err := result.NormalizeAndValidate(); err != nil {
		return AgentResult{}, err
	}
	return result, nil
}

type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
)

type Message struct {
	ID           string      `json:"id"`
	ProjectID    string      `json:"projectId"`
	Role         MessageRole `json:"role"`
	Content      string      `json:"content"`
	GenerationID *string     `json:"generationId,omitzero"`
	CreatedAt    time.Time   `json:"createdAt"`
}

type GenerationAttempt struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"startedAt"`
}

type ArtifactManifest struct {
	Template AppTemplate `json:"template"`
	Actions  []string    `json:"actions"`
	Checksum string      `json:"checksum"`
}

type CompiledArtifact struct {
	EntryHTML string           `json:"entryHtml"`
	HTML      string           `json:"html"`
	CSS       string           `json:"css"`
	JS        string           `json:"js"`
	Manifest  ArtifactManifest `json:"manifest"`
}

type TodoPreviewState struct {
	Items []TodoPreviewItem `json:"items"`
}

type TodoPreviewItem struct {
	ID       string       `json:"id"`
	Text     string       `json:"text"`
	Category string       `json:"category"`
	Priority TodoPriority `json:"priority"`
	Done     bool         `json:"done"`
}

type NotesPreviewState struct {
	Notes []NotePreviewItem `json:"notes"`
}

type NotePreviewItem struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

type HabitsPreviewState struct {
	Habits []HabitPreviewItem `json:"habits"`
}

type HabitPreviewItem struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Icon           HabitIcon `json:"icon"`
	TargetPerWeek  int       `json:"targetPerWeek"`
	CompletedDates []string  `json:"completedDates"`
}

// NormalizePreviewState accepts only the small, template-specific state that
// an iframe can publish. It returns normalized JSON ready for SQLite storage.
func NormalizePreviewState(spec AppSpec, raw json.RawMessage) (json.RawMessage, error) {
	if err := spec.NormalizeAndValidate(); err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || len(trimmed) > 64<<10 || bytes.Equal(trimmed, []byte("null")) {
		return nil, fmt.Errorf("%w: state must be a non-empty object under 64 KiB", ErrInvalidPreviewState)
	}

	switch spec.TemplateName() {
	case TemplateTodo:
		var state TodoPreviewState
		if err := decodeStrictJSON(trimmed, &state); err != nil {
			return nil, fmt.Errorf("%w: decode todo state: %v", ErrInvalidPreviewState, err)
		}
		if err := state.normalizeAndValidate(*spec.Todo); err != nil {
			return nil, err
		}
		return json.Marshal(state)
	case TemplateNotes:
		var state NotesPreviewState
		if err := decodeStrictJSON(trimmed, &state); err != nil {
			return nil, fmt.Errorf("%w: decode notes state: %v", ErrInvalidPreviewState, err)
		}
		if err := state.normalizeAndValidate(*spec.Notes); err != nil {
			return nil, err
		}
		return json.Marshal(state)
	case TemplateHabits:
		var state HabitsPreviewState
		if err := decodeStrictJSON(trimmed, &state); err != nil {
			return nil, fmt.Errorf("%w: decode habits state: %v", ErrInvalidPreviewState, err)
		}
		if err := state.normalizeAndValidate(*spec.Habits); err != nil {
			return nil, err
		}
		return json.Marshal(state)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedTemplate, spec.TemplateName())
	}
}

func (s *TodoPreviewState) normalizeAndValidate(spec TodoSpec) error {
	if len(s.Items) > 100 {
		return fmt.Errorf("%w: todo state exceeds 100 items", ErrInvalidPreviewState)
	}
	categories := make(map[string]struct{}, len(spec.Categories))
	for _, category := range spec.Categories {
		categories[strings.ToLower(category)] = struct{}{}
	}
	ids := make(map[string]struct{}, len(s.Items))
	for index := range s.Items {
		item := &s.Items[index]
		var err error
		if item.ID, err = normalizeAppText(item.ID, 64, true); err != nil {
			return fmt.Errorf("%w: todo item %d id %v", ErrInvalidPreviewState, index+1, err)
		}
		if _, exists := ids[item.ID]; exists {
			return fmt.Errorf("%w: todo item %d repeats id", ErrInvalidPreviewState, index+1)
		}
		ids[item.ID] = struct{}{}
		if item.Text, err = normalizeAppText(item.Text, maxTodoItemText, true); err != nil {
			return fmt.Errorf("%w: todo item %d text %v", ErrInvalidPreviewState, index+1, err)
		}
		if item.Category, err = normalizeAppText(item.Category, maxTodoCategoryLength, true); err != nil {
			return fmt.Errorf("%w: todo item %d category %v", ErrInvalidPreviewState, index+1, err)
		}
		if _, exists := categories[strings.ToLower(item.Category)]; !exists {
			return fmt.Errorf("%w: todo item %d has unknown category", ErrInvalidPreviewState, index+1)
		}
		if !isTodoPriority(item.Priority) {
			return fmt.Errorf("%w: todo item %d has unknown priority", ErrInvalidPreviewState, index+1)
		}
	}
	return nil
}

func (s *NotesPreviewState) normalizeAndValidate(spec NotesSpec) error {
	if len(s.Notes) > 100 {
		return fmt.Errorf("%w: notes state exceeds 100 notes", ErrInvalidPreviewState)
	}
	tags := make(map[string]struct{}, len(spec.Tags))
	for _, tag := range spec.Tags {
		tags[strings.ToLower(tag)] = struct{}{}
	}
	ids := make(map[string]struct{}, len(s.Notes))
	for index := range s.Notes {
		note := &s.Notes[index]
		var err error
		if note.ID, err = normalizeAppText(note.ID, 64, true); err != nil {
			return fmt.Errorf("%w: note %d id %v", ErrInvalidPreviewState, index+1, err)
		}
		if _, exists := ids[note.ID]; exists {
			return fmt.Errorf("%w: note %d repeats id", ErrInvalidPreviewState, index+1)
		}
		ids[note.ID] = struct{}{}
		if note.Title, err = normalizeAppText(note.Title, maxNoteTitle, true); err != nil {
			return fmt.Errorf("%w: note %d title %v", ErrInvalidPreviewState, index+1, err)
		}
		if note.Content, err = normalizeAppText(note.Content, maxNoteContent, false); err != nil {
			return fmt.Errorf("%w: note %d content %v", ErrInvalidPreviewState, index+1, err)
		}
		if len(note.Tags) > 3 {
			return fmt.Errorf("%w: note %d has too many tags", ErrInvalidPreviewState, index+1)
		}
		seenTags := make(map[string]struct{}, len(note.Tags))
		for tagIndex, tag := range note.Tags {
			normalized, err := normalizeAppText(tag, maxNotesTagLength, true)
			if err != nil {
				return fmt.Errorf("%w: note %d tag %d %v", ErrInvalidPreviewState, index+1, tagIndex+1, err)
			}
			key := strings.ToLower(normalized)
			if _, exists := tags[key]; !exists {
				return fmt.Errorf("%w: note %d has unknown tag", ErrInvalidPreviewState, index+1)
			}
			if _, exists := seenTags[key]; exists {
				return fmt.Errorf("%w: note %d repeats a tag", ErrInvalidPreviewState, index+1)
			}
			seenTags[key] = struct{}{}
			note.Tags[tagIndex] = normalized
		}
	}
	return nil
}

func (s *HabitsPreviewState) normalizeAndValidate(spec HabitsSpec) error {
	if len(s.Habits) > 100 {
		return fmt.Errorf("%w: habits state exceeds 100 habits", ErrInvalidPreviewState)
	}
	ids := make(map[string]struct{}, len(s.Habits))
	for index := range s.Habits {
		habit := &s.Habits[index]
		var err error
		if habit.ID, err = normalizeAppText(habit.ID, 64, true); err != nil {
			return fmt.Errorf("%w: habit %d id %v", ErrInvalidPreviewState, index+1, err)
		}
		if _, exists := ids[habit.ID]; exists {
			return fmt.Errorf("%w: habit %d repeats id", ErrInvalidPreviewState, index+1)
		}
		ids[habit.ID] = struct{}{}
		if habit.Name, err = normalizeAppText(habit.Name, maxHabitName, true); err != nil {
			return fmt.Errorf("%w: habit %d name %v", ErrInvalidPreviewState, index+1, err)
		}
		if !isHabitIcon(habit.Icon) {
			return fmt.Errorf("%w: habit %d has unknown icon", ErrInvalidPreviewState, index+1)
		}
		if habit.TargetPerWeek < 1 || habit.TargetPerWeek > 7 {
			return fmt.Errorf("%w: habit %d target must be 1–7", ErrInvalidPreviewState, index+1)
		}
		if len(habit.CompletedDates) > 366 {
			return fmt.Errorf("%w: habit %d has too many completed dates", ErrInvalidPreviewState, index+1)
		}
		seenDates := make(map[string]struct{}, len(habit.CompletedDates))
		for dateIndex, date := range habit.CompletedDates {
			parsed, err := time.Parse("2006-01-02", date)
			if err != nil || parsed.Format("2006-01-02") != date {
				return fmt.Errorf("%w: habit %d completed date %d is invalid", ErrInvalidPreviewState, index+1, dateIndex+1)
			}
			if _, exists := seenDates[date]; exists {
				return fmt.Errorf("%w: habit %d repeats a completed date", ErrInvalidPreviewState, index+1)
			}
			seenDates[date] = struct{}{}
		}
	}
	return nil
}

type GenerationVersion struct {
	ID              string           `json:"id"`
	ProjectID       string           `json:"projectId"`
	ParentVersionID *string          `json:"parentVersionId,omitzero"`
	Sequence        int              `json:"sequence"`
	Request         string           `json:"request"`
	Plan            AgentPlan        `json:"plan"`
	Spec            AppSpec          `json:"spec"`
	Artifact        CompiledArtifact `json:"artifact"`
	CreatedAt       time.Time        `json:"createdAt"`
}

func NormalizeGenerationRequest(value string) (string, error) {
	normalized, err := normalizeAppText(value, maxGenerationRequest, true)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidGenerationRequest, err)
	}
	return normalized, nil
}

func normalizeAppText(value string, maximumLength int, required bool) (string, error) {
	normalized := strings.TrimSpace(value)
	if required && normalized == "" {
		return "", errors.New("must not be empty")
	}
	if utf8.RuneCountInString(normalized) > maximumLength {
		return "", fmt.Errorf("must contain at most %d characters", maximumLength)
	}
	if strings.ContainsRune(normalized, '\x00') {
		return "", errors.New("must not contain NUL characters")
	}
	return normalized, nil
}

func decodeStrictJSON(data []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("JSON must contain a single object")
	}
	return nil
}
