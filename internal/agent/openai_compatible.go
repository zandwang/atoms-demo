package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/zand/atoms-demo/internal/domain"
)

const maxModelResponseSize = 512 << 10

type OpenAICompatibleAdapter struct {
	baseURL      string
	apiKey       string
	model        string
	allowPrivate bool
	client       *http.Client
}

func NewOpenAICompatibleAdapter(baseURL, apiKey, model string) *OpenAICompatibleAdapter {
	return NewOpenAICompatibleAdapterWithOptions(baseURL, apiKey, model, true)
}

func NewOpenAICompatibleAdapterWithOptions(baseURL, apiKey, model string, allowPrivate bool) *OpenAICompatibleAdapter {
	validatedURL, _ := ValidateBaseURL(baseURL)
	return &OpenAICompatibleAdapter{
		baseURL:      validatedURL,
		apiKey:       strings.TrimSpace(apiKey),
		model:        strings.TrimSpace(model),
		allowPrivate: allowPrivate,
		client: &http.Client{Transport: &http.Transport{
			Proxy: nil,
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				return safeDialContext(ctx, network, address, allowPrivate)
			},
		}},
	}
}

func (a *OpenAICompatibleAdapter) Generate(ctx context.Context, input PromptInput) (domain.AgentResult, error) {
	if a == nil || a.baseURL == "" || a.apiKey == "" || a.model == "" {
		return domain.AgentResult{}, &Error{Code: ErrorUpstreamUnavailable, Message: "模型服务尚未就绪，请检查本地配置。", Retryable: true}
	}
	if err := validateEndpointNetwork(ctx, a.baseURL, a.allowPrivate); err != nil {
		return domain.AgentResult{}, err
	}
	messages, err := buildChatMessages(input)
	if err != nil {
		return domain.AgentResult{}, PublicError(err)
	}

	content, status, err := a.request(ctx, messages, true)
	if err != nil {
		return domain.AgentResult{}, err
	}
	if status == http.StatusBadRequest || status == http.StatusUnprocessableEntity {
		// Some OpenAI-compatible endpoints do not implement response_format.
		// Retry once without it, then rely on strict local JSON validation.
		content, _, err = a.request(ctx, messages, false)
		if err != nil {
			return domain.AgentResult{}, err
		}
	}

	result, err := domain.ParseAgentResult([]byte(content))
	if err != nil {
		return domain.AgentResult{}, PublicError(err)
	}
	return result, nil
}

type chatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (a *OpenAICompatibleAdapter) request(ctx context.Context, messages []chatMessage, useJSONMode bool) (string, int, error) {
	payload := chatCompletionRequest{
		Model:       a.model,
		Messages:    messages,
		Temperature: 0.2,
	}
	if useJSONMode {
		payload.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", 0, &Error{Code: ErrorUpstreamUnavailable, Message: "无法准备模型请求，请重试。", Retryable: true, Cause: err}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", 0, &Error{Code: ErrorUpstreamUnavailable, Message: "模型地址无效，请检查本地配置。", Retryable: true, Cause: err}
	}
	request.Header.Set("Authorization", "Bearer "+a.apiKey)
	request.Header.Set("Content-Type", "application/json")

	response, err := a.client.Do(request)
	if err != nil {
		var adapterError *Error
		if errors.As(err, &adapterError) {
			return "", 0, adapterError
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", 0, &Error{Code: ErrorUpstreamTimeout, Message: "模型响应超时，请重试。", Retryable: true, Cause: err}
		}
		return "", 0, &Error{Code: ErrorUpstreamUnavailable, Message: "模型服务暂时不可用，请稍后重试。", Retryable: true, Cause: err}
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		// Deliberately consume only a bounded amount and never expose or log it:
		// upstream bodies can contain provider-specific details.
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxModelResponseSize))
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			return "", response.StatusCode, &Error{Code: ErrorAuth, Message: "模型服务拒绝了这个 API Key，请更新后重试。", Retryable: true}
		}
		if response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError {
			return "", response.StatusCode, &Error{Code: ErrorUpstreamUnavailable, Message: "模型服务暂时不可用，请稍后重试。", Retryable: true}
		}
		// A few compatible APIs reject JSON mode with 400 or 422. Only that
		// first, optional feature probe gets a fallback; all other client-side
		// failures are surfaced as a safe configuration/upstream error.
		if useJSONMode && (response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusUnprocessableEntity) {
			return "", response.StatusCode, nil
		}
		return "", response.StatusCode, &Error{Code: ErrorUpstreamUnavailable, Message: "模型接口拒绝了请求，请检查本地配置后重试。", Retryable: true}
	}

	var decoded chatCompletionResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxModelResponseSize))
	if err := decoder.Decode(&decoded); err != nil {
		return "", response.StatusCode, &Error{Code: ErrorModelOutputInvalid, Message: "模型返回内容无法解析，请重试。", Retryable: true, Cause: err}
	}
	if len(decoded.Choices) == 0 {
		return "", response.StatusCode, &Error{Code: ErrorModelOutputInvalid, Message: "模型没有返回可用结果，请重试。", Retryable: true}
	}
	var content string
	if err := json.Unmarshal(decoded.Choices[0].Message.Content, &content); err != nil || strings.TrimSpace(content) == "" {
		return "", response.StatusCode, &Error{Code: ErrorModelOutputInvalid, Message: "模型没有返回可用 JSON 结果，请重试。", Retryable: true, Cause: err}
	}
	return content, response.StatusCode, nil
}
