package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zand/atoms-demo/internal/agent"
	"github.com/zand/atoms-demo/internal/compiler"
	"github.com/zand/atoms-demo/internal/domain"
)

const (
	modelBaseURLHeader    = "X-Model-Base-URL"
	modelNameHeader       = "X-Model-Name"
	modelAPIKeyHeader     = "X-Model-API-Key"
	maxModelBaseURLLength = 2048
	maxModelNameLength    = 256
	maxModelAPIKeyLength  = 4096
)

var errModelGenerationTimeout = errors.New("model generation deadline exceeded")

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
	generationStartedAt := time.Now()
	s.logger.Info("generation started",
		"project_id", projectID,
		"attempt_id", attempt.ID,
		"request_length", len(userRequest),
		"recent_message_count", len(recentMessages),
		"has_current_version", currentVersion != nil,
	)

	prepareSSE(w)
	if err := writeSSE(w, "stage", generationStageEvent{Status: "requesting_model", Label: "正在请求模型…"}); err != nil {
		return
	}
	modelStartedAt := time.Now()
	modelTimeout := s.config.GenerationTimeout()
	s.logger.Info("model generation started",
		"project_id", projectID,
		"attempt_id", attempt.ID,
		"timeout", modelTimeout,
		"byok", s.model == nil,
	)
	modelContext, cancel := context.WithTimeoutCause(r.Context(), modelTimeout, errModelGenerationTimeout)
	defer cancel()
	result, err := model.Generate(modelContext, agent.PromptInput{
		ProjectName:    project.Name,
		UserRequest:    userRequest,
		CurrentSpec:    currentSpec(currentVersion),
		RecentMessages: recentMessages,
	})
	if err != nil {
		failure := agent.PublicError(err)
		s.logger.Warn("model generation failed",
			"project_id", projectID,
			"attempt_id", attempt.ID,
			"duration", time.Since(modelStartedAt),
			"error_code", failure.Code,
			"upstream_status", failure.UpstreamStatus,
			"cause", diagnosticCause(err),
		)
		sendGenerationFailure(s, w, workspace, projectID, attempt.ID, failure)
		return
	}
	htmlBytes, cssBytes, jsBytes := 0, 0, 0
	if result.Spec.Files != nil {
		htmlBytes = len(result.Spec.Files.HTML)
		cssBytes = len(result.Spec.Files.CSS)
		jsBytes = len(result.Spec.Files.JS)
	}
	s.logger.Info("model generation completed",
		"project_id", projectID,
		"attempt_id", attempt.ID,
		"duration", time.Since(modelStartedAt),
		"html_bytes", htmlBytes,
		"css_bytes", cssBytes,
		"js_bytes", jsBytes,
	)
	if err := writeSSE(w, "stage", generationStageEvent{Status: "validating", Label: "正在校验应用规格…"}); err != nil {
		return
	}
	if err := result.NormalizeAndValidate(); err != nil {
		s.logger.Warn("model result validation failed",
			"project_id", projectID,
			"attempt_id", attempt.ID,
			"duration", time.Since(generationStartedAt),
			"cause", diagnosticCause(err),
		)
		sendGenerationFailure(s, w, workspace, projectID, attempt.ID, agent.PublicError(err))
		return
	}
	if err := writeSSE(w, "stage", generationStageEvent{Status: "compiling", Label: "正在编译可运行预览…"}); err != nil {
		return
	}
	artifact, err := compiler.Compile(result.Spec)
	if err != nil {
		s.logger.Warn("generated app compilation failed",
			"project_id", projectID,
			"attempt_id", attempt.ID,
			"duration", time.Since(generationStartedAt),
			"cause", diagnosticCause(err),
		)
		sendGenerationFailure(s, w, workspace, projectID, attempt.ID, agent.PublicError(err))
		return
	}
	version, err := s.repository.CompleteGeneration(r.Context(), workspace.ID, projectID, attempt.ID, activeVersionID(currentVersion), result, artifact)
	if err != nil {
		s.logger.Error("complete generation failed", "project_id", projectID, "attempt_id", attempt.ID, "error", diagnosticCause(err))
		failure := &agent.Error{Code: agent.ErrorUpstreamUnavailable, Message: "无法保存生成结果，请重试。", Retryable: true, Cause: err}
		sendGenerationFailure(s, w, workspace, projectID, attempt.ID, failure)
		return
	}
	s.logger.Info("generation completed", "project_id", projectID, "attempt_id", attempt.ID, "version_id", version.ID, "duration", time.Since(generationStartedAt))
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
		s.logger.Error("record generation failure", "project_id", projectID, "attempt_id", attemptID, "error", diagnosticCause(err))
	}
	s.logger.Warn("generation failed", "project_id", projectID, "attempt_id", attemptID, "error_code", failure.Code, "upstream_status", failure.UpstreamStatus, "retryable", failure.Retryable, "cause", diagnosticCause(failure.Cause))
	_ = writeSSE(w, "error", generationErrorEvent{Code: string(failure.Code), Message: failure.Message, Retryable: failure.Retryable})
}

func diagnosticCause(err error) string {
	if err == nil {
		return "not available"
	}
	if errors.Is(err, errModelGenerationTimeout) {
		return errModelGenerationTimeout.Error()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "context deadline exceeded"
	}
	if errors.Is(err, context.Canceled) {
		return "context canceled"
	}
	var modelError *agent.Error
	if errors.As(err, &modelError) && modelError.Cause != nil && modelError.Cause != err {
		return diagnosticCause(modelError.Cause)
	}
	var urlError *url.Error
	if errors.As(err, &urlError) {
		return fmt.Sprintf("HTTP %s failed: %s", urlError.Op, diagnosticCause(urlError.Err))
	}
	var dnsError *net.DNSError
	if errors.As(err, &dnsError) {
		return fmt.Sprintf("DNS lookup failed: timeout=%t not_found=%t", dnsError.IsTimeout, dnsError.IsNotFound)
	}
	var operationError *net.OpError
	if errors.As(err, &operationError) {
		return fmt.Sprintf("network %s failed: %s", operationError.Op, diagnosticCause(operationError.Err))
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return fmt.Sprintf("network error: type=%T timeout=%t", networkError, networkError.Timeout())
	}
	return fmt.Sprintf("type=%T", err)
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
