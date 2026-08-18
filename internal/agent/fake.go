package agent

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/zand/atoms-demo/internal/domain"
)

// FakeAdapter is intentionally explicit and is used only by tests and local
// development wiring supplied by a caller. It is never selected as a silent
// fallback when a real model call fails.
type FakeAdapter struct {
	Result domain.AgentResult
	Err    error

	mu     sync.Mutex
	Inputs []PromptInput
}

func (f *FakeAdapter) Generate(ctx context.Context, input PromptInput) (domain.AgentResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.AgentResult{}, err
	}
	f.mu.Lock()
	f.Inputs = append(f.Inputs, input)
	f.mu.Unlock()
	if f.Err != nil {
		return domain.AgentResult{}, f.Err
	}
	encoded, err := json.Marshal(f.Result)
	if err != nil {
		return domain.AgentResult{}, err
	}
	return domain.ParseAgentResult(encoded)
}
