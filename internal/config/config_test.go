package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
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
	cfg := Config{OpenAIBaseURL: "https://example.test/v1", OpenAIAPIKey: "key"}
	status := cfg.ModelStatus()
	if status.Configured {
		t.Fatal("ModelStatus().Configured = true, want false")
	}
	if !reflect.DeepEqual(status.Missing, []string{"OPENAI_MODEL"}) {
		t.Fatalf("ModelStatus().Missing = %#v", status.Missing)
	}
}
