package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestReadEnvFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), ".env")
	contents := "# comment\nOPENAI_BASE_URL=https://example.test/v1\nOPENAI_API_KEY=quoted value\nOPENAI_MODEL='demo-model'\n"
	if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	values, err := readEnvFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"OPENAI_BASE_URL": "https://example.test/v1",
		"OPENAI_API_KEY":  "quoted value",
		"OPENAI_MODEL":    "demo-model",
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("readEnvFile() = %#v, want %#v", values, want)
	}
}

func TestModelStatus(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want ModelStatus
	}{
		{
			name: "empty config supports browser settings",
			want: ModelStatus{Available: true},
		},
		{
			name: "default provider without key",
			cfg:  Config{ModelBaseURL: "https://default.example/v1", ModelName: "default-model"},
			want: ModelStatus{Available: true, DefaultProviderConfigured: true},
		},
		{
			name: "complete default model",
			cfg:  Config{ModelBaseURL: "https://default.example/v1", ModelName: "default-model", ModelAPIKey: "default-key"},
			want: ModelStatus{Available: true, DefaultProviderConfigured: true, DefaultConfigured: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.cfg.ModelStatus(); got != test.want {
				t.Fatalf("ModelStatus() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestLoadDefaultModelConfiguration(t *testing.T) {
	filename := filepath.Join(t.TempDir(), ".env")
	contents := "OPENAI_BASE_URL=https://default.example/v1\nOPENAI_MODEL=default-model\nOPENAI_API_KEY=default-key\n"
	if err := os.WriteFile(filename, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ModelBaseURL != "https://default.example/v1" || cfg.ModelName != "default-model" || cfg.ModelAPIKey != "default-key" {
		t.Fatal("Load() did not read the complete default model configuration")
	}
}

func TestLoadPrivateEndpointFlag(t *testing.T) {
	filename := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(filename, []byte("ATOMS_ALLOW_PRIVATE_MODEL_ENDPOINTS=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AllowPrivateModelEndpoint {
		t.Fatal("AllowPrivateModelEndpoint = false, want true")
	}
}

func TestLoadUsesPlatformPortWhenAddressIsUnset(t *testing.T) {
	filename := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(filename, []byte("PORT=10000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Address != ":10000" {
		t.Fatalf("Address = %q, want :10000", cfg.Address)
	}
}

func TestLoadModelTimeout(t *testing.T) {
	filename := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(filename, []byte("ATOMS_MODEL_TIMEOUT=3m\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(filename)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GenerationTimeout() != 3*time.Minute {
		t.Fatalf("GenerationTimeout() = %s, want 3m", cfg.GenerationTimeout())
	}
}

func TestModelTimeoutDefaultsAndRejectsInvalidValues(t *testing.T) {
	if got := (Config{}).GenerationTimeout(); got != 120*time.Second {
		t.Fatalf("zero Config GenerationTimeout() = %s, want 2m", got)
	}
	for _, value := range []string{"invalid", "5s", "11m"} {
		if _, err := modelTimeoutFromEnv(value); err == nil {
			t.Fatalf("modelTimeoutFromEnv(%q) accepted invalid value", value)
		}
	}
}
