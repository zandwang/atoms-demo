package agent

import (
	"context"
	"strings"
	"testing"
)

func TestValidateBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		valid   bool
		expects string
	}{
		{name: "normalizes trailing slash", value: " https://provider.example/v1/ ", valid: true, expects: "https://provider.example/v1"},
		{name: "rejects missing scheme", value: "provider.example/v1"},
		{name: "rejects credentials", value: "https://user:pass@provider.example/v1"},
		{name: "rejects query", value: "https://provider.example/v1?token=secret"},
		{name: "rejects fragment", value: "https://provider.example/v1#chat"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ValidateBaseURL(test.value)
			if test.valid {
				if err != nil || got != test.expects {
					t.Fatalf("ValidateBaseURL() = %q, %v; want %q, nil", got, err, test.expects)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "endpoint") {
				t.Fatalf("ValidateBaseURL() error = %v, want endpoint error", err)
			}
		})
	}
}

func TestPrivateEndpointRejectedByDefault(t *testing.T) {
	if err := validateEndpointNetwork(context.Background(), "http://127.0.0.1:8080/v1", false); err == nil || !strings.Contains(err.Error(), "禁止") {
		t.Fatalf("validateEndpointNetwork() = %v, want private endpoint rejection", err)
	}
}

func TestPrivateEndpointCanBeEnabledExplicitly(t *testing.T) {
	if err := validateEndpointNetwork(context.Background(), "http://127.0.0.1:8080/v1", true); err != nil {
		t.Fatalf("validateEndpointNetwork() = %v, want nil", err)
	}
}
