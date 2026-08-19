// Package agent owns the model boundary. It returns validated domain data and
// never writes to storage or exposes provider responses to HTTP clients.
package agent

import (
	"context"
	"errors"

	"github.com/zand/atoms-demo/internal/domain"
)

type PromptInput struct {
	ProjectName    string
	UserRequest    string
	CurrentSpec    *domain.AppSpec
	RecentMessages []domain.Message
}

type ModelAdapter interface {
	Generate(context.Context, PromptInput) (domain.AgentResult, error)
}

type ErrorCode string

const (
	ErrorAuth                ErrorCode = "AUTH_ERROR"
	ErrorEndpointInvalid     ErrorCode = "MODEL_ENDPOINT_INVALID"
	ErrorUpstreamTimeout     ErrorCode = "UPSTREAM_TIMEOUT"
	ErrorUpstreamUnavailable ErrorCode = "UPSTREAM_UNAVAILABLE"
	ErrorModelOutputInvalid  ErrorCode = "MODEL_OUTPUT_INVALID"
	ErrorUnsupportedRequest  ErrorCode = "UNSUPPORTED_REQUEST"
)

// Error contains a safe, user-facing description and preserves its root cause
// for local control flow only. It must not be populated with raw provider data.
type Error struct {
	Code           ErrorCode
	Message        string
	Retryable      bool
	UpstreamStatus int
	Cause          error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func PublicError(err error) *Error {
	var modelError *Error
	if errors.As(err, &modelError) {
		return modelError
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Code: ErrorUpstreamTimeout, Message: "模型响应超时，请重试。", Retryable: true, Cause: err}
	}
	if errors.Is(err, domain.ErrUnsupportedTemplate) {
		return &Error{Code: ErrorUnsupportedRequest, Message: "当前仅支持可离线运行的小型单页应用，请缩小需求范围后重试。", Retryable: true, Cause: err}
	}
	if errors.Is(err, domain.ErrInvalidAgentResult) || errors.Is(err, domain.ErrInvalidAppSpec) {
		return &Error{Code: ErrorModelOutputInvalid, Message: "模型返回的应用规格无法安全使用，请重试。", Retryable: true, Cause: err}
	}
	return &Error{Code: ErrorUpstreamUnavailable, Message: "模型服务暂时不可用，请稍后重试。", Retryable: true, Cause: err}
}
