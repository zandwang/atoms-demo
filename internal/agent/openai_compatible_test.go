package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/zand/atoms-demo/internal/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func validResultJSON(t *testing.T) string {
	t.Helper()
	result := domain.AgentResult{
		Plan: domain.AgentPlan{
			Summary:          "创建一个工作清单。",
			Steps:            []string{"建立工作分类", "添加首个任务"},
			SelectedTemplate: domain.TemplateTodo,
		},
		AssistantMessage: "已生成待办清单。",
		Spec: domain.AppSpec{Todo: &domain.TodoSpec{
			SchemaVersion: 1,
			Template:      domain.TemplateTodo,
			Title:         "工作清单",
			Description:   "专注完成重要工作。",
			Theme:         domain.TodoThemeViolet,
			Categories:    []string{"工作"},
			InitialItems: []domain.TodoSeedItem{
				{Text: "完成方案", Category: "工作", Priority: domain.TodoPriorityHigh},
			},
		}},
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

	result, err := adapter.Generate(context.Background(), PromptInput{ProjectName: "Demo", UserRequest: "做一个工作待办"})
	if err != nil {
		t.Fatal(err)
	}
	if receivedAuthorization != "Bearer test-key" {
		t.Fatalf("authorization = %q", receivedAuthorization)
	}
	if result.Spec.Todo.Title != "工作清单" {
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
