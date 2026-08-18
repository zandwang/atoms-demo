package app

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zand/atoms-demo/internal/agent"
	"github.com/zand/atoms-demo/internal/config"
	"github.com/zand/atoms-demo/internal/domain"
	"github.com/zand/atoms-demo/internal/store/sqlite"
)

func TestHealthReportsMissingModelConfigurationWithoutSecrets(t *testing.T) {
	handler := NewHandler(config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	request := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var response struct {
		Data healthData `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.Model.Configured {
		t.Fatal("health reported a configured model")
	}
	if len(response.Data.Model.Missing) != 3 {
		t.Fatalf("missing settings = %#v", response.Data.Model.Missing)
	}
}

func TestGeneratePlaceholderReturnsConfigurationError(t *testing.T) {
	handler := newTestHandler(t)
	cookie := initializeSession(t, handler, "Zand")
	request := httptest.NewRequest(http.MethodPost, "/api/projects/project-1/generate", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}

	var response struct {
		Error apiError `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "CONFIG_ERROR" {
		t.Fatalf("error code = %q, want CONFIG_ERROR", response.Error.Code)
	}
}

func TestRootServesFallbackBeforeFrontendBuild(t *testing.T) {
	handler := NewHandler(config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); body == "" || !contains(body, "Atoms Demo") {
		t.Fatalf("root body = %q", body)
	}
}

func TestSessionAndProjectAPI(t *testing.T) {
	handler := newTestHandler(t)
	cookie := initializeSession(t, handler, "Zand")

	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"My app"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	createRecorder := httptest.NewRecorder()
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRecorder.Code, http.StatusCreated)
	}

	var created struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Data.Name != "My app" || created.Data.ID == "" {
		t.Fatalf("created project = %#v", created.Data)
	}

	renameRequest := httptest.NewRequest(http.MethodPatch, "/api/projects/"+created.Data.ID, strings.NewReader(`{"name":"Renamed app"}`))
	renameRequest.Header.Set("Content-Type", "application/json")
	renameRequest.AddCookie(cookie)
	renameRecorder := httptest.NewRecorder()
	handler.ServeHTTP(renameRecorder, renameRequest)
	if renameRecorder.Code != http.StatusOK {
		t.Fatalf("rename status = %d, want %d", renameRecorder.Code, http.StatusOK)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	listRequest.AddCookie(cookie)
	listRecorder := httptest.NewRecorder()
	handler.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRecorder.Code, http.StatusOK)
	}
	var listed struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Data) != 1 || listed.Data[0].Name != "Renamed app" {
		t.Fatalf("listed projects = %#v", listed.Data)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/projects/"+created.Data.ID, nil)
	deleteRequest.AddCookie(cookie)
	deleteRecorder := httptest.NewRecorder()
	handler.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d", deleteRecorder.Code, http.StatusOK)
	}
}

func TestGenerateStreamsStagesAndPersistsVersion(t *testing.T) {
	fake := &agent.FakeAdapter{Result: testGenerationResult()}
	handler := newTestHandlerWithModel(t, fake)
	cookie := initializeSession(t, handler, "Zand")

	createRequest := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"Generated app"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	createRecorder := httptest.NewRecorder()
	handler.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	generationRequest := httptest.NewRequest(http.MethodPost, "/api/projects/"+created.Data.ID+"/generate", strings.NewReader(`{"userRequest":"做一个工作待办"}`))
	generationRequest.Header.Set("Content-Type", "application/json")
	generationRequest.AddCookie(cookie)
	generationRecorder := httptest.NewRecorder()
	handler.ServeHTTP(generationRecorder, generationRequest)
	if generationRecorder.Code != http.StatusOK {
		t.Fatalf("generation status = %d; body=%s", generationRecorder.Code, generationRecorder.Body.String())
	}
	if contentType := generationRecorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("content type = %q", contentType)
	}
	for _, event := range []string{"event: stage", "requesting_model", "validating", "compiling", "event: result"} {
		if !strings.Contains(generationRecorder.Body.String(), event) {
			t.Fatalf("SSE stream missing %q: %s", event, generationRecorder.Body.String())
		}
	}
	if len(fake.Inputs) != 1 || fake.Inputs[0].UserRequest != "做一个工作待办" {
		t.Fatalf("fake inputs = %#v", fake.Inputs)
	}

	versionsRequest := httptest.NewRequest(http.MethodGet, "/api/projects/"+created.Data.ID+"/versions", nil)
	versionsRequest.AddCookie(cookie)
	versionsRecorder := httptest.NewRecorder()
	handler.ServeHTTP(versionsRecorder, versionsRequest)
	if versionsRecorder.Code != http.StatusOK || !strings.Contains(versionsRecorder.Body.String(), "工作清单") {
		t.Fatalf("versions response = %d %s", versionsRecorder.Code, versionsRecorder.Body.String())
	}
	messagesRequest := httptest.NewRequest(http.MethodGet, "/api/projects/"+created.Data.ID+"/messages", nil)
	messagesRequest.AddCookie(cookie)
	messagesRecorder := httptest.NewRecorder()
	handler.ServeHTTP(messagesRecorder, messagesRequest)
	if messagesRecorder.Code != http.StatusOK || !strings.Contains(messagesRecorder.Body.String(), "已生成工作待办清单") {
		t.Fatalf("messages response = %d %s", messagesRecorder.Code, messagesRecorder.Body.String())
	}
}

func TestVersionActivationAndPreviewStateAPI(t *testing.T) {
	handler := newTestHandlerWithModel(t, &agent.FakeAdapter{Result: testGenerationResult()})
	cookie := initializeSession(t, handler, "Zand")
	projectID := createTestProject(t, handler, cookie, "Versioned app")
	for _, requestText := range []string{"做一个工作待办", "把它改得更简洁"} {
		request := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/generate", strings.NewReader(`{"userRequest":"`+requestText+`"}`))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(cookie)
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "event: result") {
			t.Fatalf("generation response = %d %s", recorder.Code, recorder.Body.String())
		}
	}

	versionsRequest := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/versions", nil)
	versionsRequest.AddCookie(cookie)
	versionsRecorder := httptest.NewRecorder()
	handler.ServeHTTP(versionsRecorder, versionsRequest)
	var versions struct {
		Data []struct {
			ID       string `json:"id"`
			Sequence int    `json:"sequence"`
		} `json:"data"`
	}
	if err := json.Unmarshal(versionsRecorder.Body.Bytes(), &versions); err != nil {
		t.Fatal(err)
	}
	if len(versions.Data) != 2 || versions.Data[0].Sequence != 2 {
		t.Fatalf("versions = %#v", versions)
	}
	firstVersionID := versions.Data[1].ID

	activateRequest := httptest.NewRequest(http.MethodPost, "/api/projects/"+projectID+"/versions/"+firstVersionID+"/activate", nil)
	activateRequest.AddCookie(cookie)
	activateRecorder := httptest.NewRecorder()
	handler.ServeHTTP(activateRecorder, activateRequest)
	if activateRecorder.Code != http.StatusOK || !strings.Contains(activateRecorder.Body.String(), `"activeVersionId":"`+firstVersionID+`"`) {
		t.Fatalf("activate response = %d %s", activateRecorder.Code, activateRecorder.Body.String())
	}

	validState := `{"state":{"items":[{"id":"one","text":"完成方案","category":"工作","priority":"high","done":true}]}}`
	saveRequest := httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/versions/"+firstVersionID+"/preview-state", strings.NewReader(validState))
	saveRequest.Header.Set("Content-Type", "application/json")
	saveRequest.AddCookie(cookie)
	saveRecorder := httptest.NewRecorder()
	handler.ServeHTTP(saveRecorder, saveRequest)
	if saveRecorder.Code != http.StatusOK {
		t.Fatalf("save preview state = %d %s", saveRecorder.Code, saveRecorder.Body.String())
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/projects/"+projectID+"/versions/"+firstVersionID+"/preview-state", nil)
	getRequest.AddCookie(cookie)
	getRecorder := httptest.NewRecorder()
	handler.ServeHTTP(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK || !strings.Contains(getRecorder.Body.String(), `"done":true`) {
		t.Fatalf("get preview state = %d %s", getRecorder.Code, getRecorder.Body.String())
	}

	invalidState := `{"state":{"items":[{"id":"one","text":"完成方案","category":"未知","priority":"high","done":true}]}}`
	invalidRequest := httptest.NewRequest(http.MethodPut, "/api/projects/"+projectID+"/versions/"+firstVersionID+"/preview-state", strings.NewReader(invalidState))
	invalidRequest.Header.Set("Content-Type", "application/json")
	invalidRequest.AddCookie(cookie)
	invalidRecorder := httptest.NewRecorder()
	handler.ServeHTTP(invalidRecorder, invalidRequest)
	if invalidRecorder.Code != http.StatusBadRequest || !strings.Contains(invalidRecorder.Body.String(), `"code":"PREVIEW_STATE_INVALID"`) {
		t.Fatalf("invalid preview state = %d %s", invalidRecorder.Code, invalidRecorder.Body.String())
	}
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	repository, err := sqlite.OpenPath(filepath.Join(t.TempDir(), "atoms-demo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := repository.Close(); err != nil {
			t.Error(err)
		}
	})
	return NewHandler(config.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)), repository)
}

func newTestHandlerWithModel(t *testing.T, model agent.ModelAdapter) http.Handler {
	t.Helper()
	repository, err := sqlite.OpenPath(filepath.Join(t.TempDir(), "atoms-demo.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := repository.Close(); err != nil {
			t.Error(err)
		}
	})
	return NewHandlerWithModel(config.Config{
		OpenAIBaseURL: "https://model.example/v1",
		OpenAIAPIKey:  "test-key",
		OpenAIModel:   "test-model",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), repository, model)
}

func initializeSession(t *testing.T, handler http.Handler, displayName string) *http.Cookie {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/session/initialize", strings.NewReader(`{"displayName":"`+displayName+`"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("initialize status = %d, want %d; body=%s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %#v", cookies)
	}
	return cookies[0]
}

func createTestProject(t *testing.T, handler http.Handler, cookie *http.Cookie, name string) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/projects", strings.NewReader(`{"name":"`+name+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create project status = %d; body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.ID == "" {
		t.Fatal("created project has no ID")
	}
	return response.Data.ID
}

func contains(text, fragment string) bool {
	for index := range len(text) {
		if len(text)-index >= len(fragment) && text[index:index+len(fragment)] == fragment {
			return true
		}
	}
	return false
}

func testGenerationResult() domain.AgentResult {
	return domain.AgentResult{
		Plan: domain.AgentPlan{
			Summary:          "创建一个工作待办清单。",
			Steps:            []string{"建立工作分类", "添加初始任务"},
			SelectedTemplate: domain.TemplateTodo,
		},
		AssistantMessage: "已生成工作待办清单。",
		Spec: domain.AppSpec{Todo: &domain.TodoSpec{
			SchemaVersion: 1,
			Template:      domain.TemplateTodo,
			Title:         "工作清单",
			Description:   "今天专注完成关键任务。",
			Theme:         domain.TodoThemeViolet,
			Categories:    []string{"工作"},
			InitialItems: []domain.TodoSeedItem{
				{Text: "完成方案", Category: "工作", Priority: domain.TodoPriorityHigh},
			},
		}},
	}
}
