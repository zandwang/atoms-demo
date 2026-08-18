package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zand/atoms-demo/internal/agent"
	"github.com/zand/atoms-demo/internal/compiler"
	"github.com/zand/atoms-demo/internal/domain"
)

const (
	generationTimeout     = 45 * time.Second
	modelBaseURLHeader    = "X-Model-Base-URL"
	modelNameHeader       = "X-Model-Name"
	modelAPIKeyHeader     = "X-Model-API-Key"
	maxModelBaseURLLength = 2048
	maxModelNameLength    = 256
	maxModelAPIKeyLength  = 4096
)

type generateRequest struct {
	UserRequest string `json:"userRequest"`
}

type generationStageEvent struct {
	Status string `json:"status"`
	Label  string `json:"label"`
}

type generationResultEvent struct {
	Version domain.GenerationVersion `json:"version"`
	Message string                   `json:"message"`
}

type generationErrorEvent struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (s server) handleGenerate(w http.ResponseWriter, r *http.Request, workspace domain.Workspace) {
	model := s.model
	if model == nil {
		baseURL := strings.TrimSpace(r.Header.Get(modelBaseURLHeader))
		modelName := strings.TrimSpace(r.Header.Get(modelNameHeader))
		apiKey := strings.TrimSpace(r.Header.Get(modelAPIKeyHeader))
		if baseURL == "" || modelName == "" {
			writeError(w, http.StatusBadRequest, "MODEL_CONFIG_REQUIRED", "请先设置模型 endpoint 和 model。", false)
			return
		}
		if apiKey == "" {
			writeError(w, http.StatusBadRequest, "API_KEY_REQUIRED", "请先设置你自己的模型 API Key。", false)
			return
		}
		if len(baseURL) > maxModelBaseURLLength || len(modelName) > maxModelNameLength || len(apiKey) > maxModelAPIKeyLength {
			writeError(w, http.StatusBadRequest, "BAD_REQUEST", "模型配置格式无效。", false)
			return
		}
		validatedURL, err := agent.ValidateBaseURL(baseURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, string(agent.ErrorEndpointInvalid), "模型 endpoint 无效，请检查地址后重试。", false)
			return
		}
		model = agent.NewOpenAICompatibleAdapterWithOptions(validatedURL, apiKey, modelName, s.config.AllowPrivateModelEndpoint)
	}

	var request generateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请输入有效的应用需求。", false)
		return
	}
	userRequest, err := domain.NormalizeGenerationRequest(request.UserRequest)
	if errors.Is(err, domain.ErrInvalidGenerationRequest) {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "需求长度需为 1–2,000 个字符。", false)
		return
	}
	if err != nil {
		s.logger.Error("validate generation request", "error", err)
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请输入有效的应用需求。", false)
		return
	}

	if !s.gate.Acquire(workspace.ID) {
		writeError(w, http.StatusConflict, "GENERATION_IN_PROGRESS", "当前工作区已有生成任务，请等待完成后再试。", true)
		return
	}
	defer s.gate.Release(workspace.ID)

	projectID := r.PathValue("projectID")
	project, err := s.repository.ProjectByID(r.Context(), workspace.ID, projectID)
	if s.writeProjectError(w, err) {
		return
	}
	recentMessages, err := s.repository.ListMessages(r.Context(), workspace.ID, projectID, 8)
	if s.writeProjectError(w, err) {
		return
	}
	currentVersion, err := s.repository.ActiveVersion(r.Context(), workspace.ID, projectID)
	if s.writeProjectError(w, err) {
		return
	}
	attempt, err := s.repository.BeginGeneration(r.Context(), workspace.ID, projectID, userRequest)
	if s.writeProjectError(w, err) {
		return
	}

	prepareSSE(w)
	if err := writeSSE(w, "stage", generationStageEvent{Status: "requesting_model", Label: "正在请求模型…"}); err != nil {
		return
	}
	modelContext, cancel := context.WithTimeoutCause(r.Context(), generationTimeout, errors.New("model generation timed out"))
	defer cancel()
	result, err := model.Generate(modelContext, agent.PromptInput{
		ProjectName:    project.Name,
		UserRequest:    userRequest,
		CurrentSpec:    currentSpec(currentVersion),
		RecentMessages: recentMessages,
	})
	if err != nil {
		sendGenerationFailure(s, w, workspace, projectID, attempt.ID, agent.PublicError(err))
		return
	}
	if err := writeSSE(w, "stage", generationStageEvent{Status: "validating", Label: "正在校验应用规格…"}); err != nil {
		return
	}
	if err := result.NormalizeAndValidate(); err != nil {
		sendGenerationFailure(s, w, workspace, projectID, attempt.ID, agent.PublicError(err))
		return
	}
	if err := writeSSE(w, "stage", generationStageEvent{Status: "compiling", Label: "正在编译可运行预览…"}); err != nil {
		return
	}
	artifact, err := compiler.Compile(result.Spec)
	if err != nil {
		sendGenerationFailure(s, w, workspace, projectID, attempt.ID, agent.PublicError(err))
		return
	}
	version, err := s.repository.CompleteGeneration(r.Context(), workspace.ID, projectID, attempt.ID, activeVersionID(currentVersion), result, artifact)
	if err != nil {
		s.logger.Error("complete generation", "project_id", projectID, "error", err)
		failure := &agent.Error{Code: agent.ErrorUpstreamUnavailable, Message: "无法保存生成结果，请重试。", Retryable: true, Cause: err}
		sendGenerationFailure(s, w, workspace, projectID, attempt.ID, failure)
		return
	}
	_ = writeSSE(w, "result", generationResultEvent{Version: version, Message: result.AssistantMessage})
}

func currentSpec(version *domain.GenerationVersion) *domain.AppSpec {
	if version == nil {
		return nil
	}
	spec := version.Spec
	return &spec
}

func activeVersionID(version *domain.GenerationVersion) *string {
	if version == nil {
		return nil
	}
	id := version.ID
	return &id
}

func sendGenerationFailure(s server, w http.ResponseWriter, workspace domain.Workspace, projectID, attemptID string, failure *agent.Error) {
	if failure == nil {
		failure = agent.PublicError(nil)
	}
	persistContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.repository.FailGeneration(persistContext, workspace.ID, projectID, attemptID, string(failure.Code), failure.Message); err != nil {
		s.logger.Error("record generation failure", "project_id", projectID, "error", err)
	}
	_ = writeSSE(w, "error", generationErrorEvent{Code: string(failure.Code), Message: failure.Message, Retryable: failure.Retryable})
}

func prepareSSE(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
}

func writeSSE(w http.ResponseWriter, event string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encode SSE event: %w", err)
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload); err != nil {
		return err
	}
	return http.NewResponseController(w).Flush()
}

type generationGate struct {
	mu     sync.Mutex
	active map[string]struct{}
}

func newGenerationGate() *generationGate {
	return &generationGate{active: make(map[string]struct{})}
}

func (g *generationGate) Acquire(workspaceID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.active[workspaceID]; exists {
		return false
	}
	g.active[workspaceID] = struct{}{}
	return true
}

func (g *generationGate) Release(workspaceID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.active, workspaceID)
}
