package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zand/atoms-demo/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func validResultJSON(t *testing.T) string {
	t.Helper()
	files := domain.GeneratedFiles{
		HTML: `<main><h1>番茄钟</h1><button id="start">开始</button></main>`,
		CSS:  `body { font-family: sans-serif; }`,
		JS:   `document.getElementById("start").addEventListener("click", () => {});`,
	}
	result := domain.AgentResult{
		Plan: domain.AgentPlan{
			Summary: "创建一个番茄钟。",
			Steps:   []string{"建立计时界面", "添加开始操作"},
		},
		AssistantMessage: "已生成番茄钟。",
		Spec:             domain.AppSpec{Files: &files},
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestOpenAICompatibleAdapterParsesValidatedResult(t *testing.T) {
	content := validResultJSON(t)
	var receivedAuthorization string
	adapter := NewOpenAICompatibleAdapter("https://model.example/v1", "test-key", "demo-model")
	adapter.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		receivedAuthorization = request.Header.Get("Authorization")
		if request.URL.String() != "https://model.example/v1/chat/completions" {
			t.Fatalf("URL = %q", request.URL)
		}
		responseBody, err := json.Marshal(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(responseBody)))}, nil
	})}

	result, err := adapter.Generate(context.Background(), PromptInput{ProjectName: "Demo", UserRequest: "做一个番茄钟"})
	if err != nil {
		t.Fatal(err)
	}
	if receivedAuthorization != "Bearer test-key" {
		t.Fatalf("authorization = %q", receivedAuthorization)
	}
	if result.Spec.TemplateName() != domain.TemplateCustom || !strings.Contains(result.Spec.Files.HTML, "番茄钟") {
		t.Fatalf("result = %#v", result)
	}
}

func TestOpenAICompatibleAdapterFallsBackWithoutJSONMode(t *testing.T) {
	content := validResultJSON(t)
	requests := 0
	adapter := NewOpenAICompatibleAdapter("https://model.example/v1", "test-key", "demo-model")
	adapter.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		payload, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatal(err)
		}
		hasJSONMode := strings.Contains(string(payload), "response_format")
		if requests == 1 {
			if !hasJSONMode {
				t.Fatal("first request should use JSON mode")
			}
			return &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":"unsupported"}`))}, nil
		}
		if hasJSONMode {
			t.Fatal("fallback request should omit JSON mode")
		}
		responseBody, err := json.Marshal(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": content}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(responseBody)))}, nil
	})}

	if _, err := adapter.Generate(context.Background(), PromptInput{ProjectName: "Demo", UserRequest: "做一个待办"}); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestOpenAICompatibleAdapterMapsInvalidModelJSON(t *testing.T) {
	adapter := NewOpenAICompatibleAdapter("https://model.example/v1", "test-key", "demo-model")
	adapter.client = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		responseBody := `{"choices":[{"message":{"content":"not JSON"}}]}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(responseBody))}, nil
	})}

	_, err := adapter.Generate(context.Background(), PromptInput{ProjectName: "Demo", UserRequest: "做一个待办"})
	public := PublicError(err)
	if public.Code != ErrorModelOutputInvalid || !public.Retryable {
		t.Fatalf("PublicError() = %#v", public)
	}
}

func TestOpenAICompatibleAdapterPreservesSafeUpstreamStatus(t *testing.T) {
	adapter := NewOpenAICompatibleAdapter("https://model.example/v1", "test-key", "demo-model")
	adapter.client = &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusTooManyRequests, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":"rate limited"}`))}, nil
	})}

	_, err := adapter.Generate(context.Background(), PromptInput{ProjectName: "Demo", UserRequest: "做一个番茄钟"})
	public := PublicError(err)
	if public.Code != ErrorUpstreamUnavailable || public.UpstreamStatus != http.StatusTooManyRequests {
		t.Fatalf("PublicError() = %#v", public)
	}
}

func TestOpenAICompatibleAdapterUsesContextCauseOnTimeout(t *testing.T) {
	adapter := NewOpenAICompatibleAdapter("https://model.example/v1", "test-key", "demo-model")
	adapter.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	ctx, cancel := context.WithTimeoutCause(context.Background(), time.Millisecond, errors.New("model generation deadline exceeded"))
	defer cancel()

	_, err := adapter.Generate(ctx, PromptInput{ProjectName: "Demo", UserRequest: "做一个番茄钟"})
	public := PublicError(err)
	if public.Code != ErrorUpstreamTimeout || public.Cause == nil || public.Cause.Error() != "model generation deadline exceeded" {
		t.Fatalf("PublicError() = %#v", public)
	}
}
