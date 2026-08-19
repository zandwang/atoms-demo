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
	cfg := Config{}
	status := cfg.ModelStatus()
	if !status.Available {
		t.Fatal("ModelStatus().Available = false, want true")
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
