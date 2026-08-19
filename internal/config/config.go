package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddress      = ":8080"
	defaultDataDir      = "./data"
	defaultModelTimeout = 120 * time.Second
	minModelTimeout     = 10 * time.Second
	maxModelTimeout     = 10 * time.Minute
)

// Config contains only runtime settings. Secret values must never be logged or returned to clients.
type Config struct {
	Address                   string
	DataDir                   string
	ModelTimeout              time.Duration
	AllowPrivateModelEndpoint bool
}

// ModelStatus is safe to expose through health and diagnostics endpoints.
type ModelStatus struct {
	Available bool `json:"available"`
}

// Load reads a local dotenv file when present. Values already present in the process environment take precedence.
func Load(envFile string) (Config, error) {
	fileValues, err := readEnvFile(envFile)
	if err != nil {
		return Config{}, err
	}

	lookup := func(key string) string {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		return fileValues[key]
	}

	modelTimeout, err := modelTimeoutFromEnv(lookup("ATOMS_MODEL_TIMEOUT"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		Address:                   addressFromEnv(lookup),
		DataDir:                   valueOrDefault(lookup("ATOMS_DATA_DIR"), defaultDataDir),
		ModelTimeout:              modelTimeout,
		AllowPrivateModelEndpoint: parseBool(lookup("ATOMS_ALLOW_PRIVATE_MODEL_ENDPOINTS")),
	}, nil
}

// GenerationTimeout returns a usable model deadline for loaded config and
// zero-value Config instances used by tests and explicit embeddings.
func (c Config) GenerationTimeout() time.Duration {
	if c.ModelTimeout <= 0 {
		return defaultModelTimeout
	}
	return c.ModelTimeout
}

func modelTimeoutFromEnv(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultModelTimeout, nil
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("ATOMS_MODEL_TIMEOUT must be a duration such as 120s or 2m: %w", err)
	}
	if duration < minModelTimeout || duration > maxModelTimeout {
		return 0, fmt.Errorf("ATOMS_MODEL_TIMEOUT must be between %s and %s", minModelTimeout, maxModelTimeout)
	}
	return duration, nil
}

func addressFromEnv(lookup func(string) string) string {
	address := strings.TrimSpace(lookup("ATOMS_ADDR"))
	if address == "" {
		port := strings.TrimSpace(lookup("PORT"))
		if port != "" {
			address = ":" + strings.TrimPrefix(port, ":")
		}
	}
	return valueOrDefault(address, defaultAddress)
}

// ModelStatus reports whether the server supports request-scoped BYOK settings.
func (c Config) ModelStatus() ModelStatus {
	return ModelStatus{Available: true}
}

func parseBool(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && parsed
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func readEnvFile(filename string) (map[string]string, error) {
	contents, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]string{}, nil
		}
		return nil, fmt.Errorf("read dotenv file: %w", err)
	}

	values := make(map[string]string)
	lineNumber := 0
	for rawLine := range strings.SplitSeq(string(contents), "\n") {
		lineNumber++
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("parse dotenv file: line %d has no '=' separator", lineNumber)
		}
		key = strings.TrimSpace(key)
		if !validEnvKey(key) {
			return nil, fmt.Errorf("parse dotenv file: line %d has an invalid variable name", lineNumber)
		}

		parsedValue, err := parseEnvValue(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("parse dotenv file: line %d: %w", lineNumber, err)
		}
		values[key] = parsedValue
	}

	return values, nil
}

func validEnvKey(key string) bool {
	if key == "" {
		return false
	}

	for index, character := range key {
		isLetter := character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
		isDigit := character >= '0' && character <= '9'
		if !(isLetter || character == '_' || index > 0 && isDigit) {
			return false
		}
	}

	return true
}

func parseEnvValue(value string) (string, error) {
	if len(value) < 2 {
		return value, nil
	}

	if value[0] == '"' {
		if value[len(value)-1] != '"' {
			return "", errors.New("unterminated double-quoted value")
		}
		parsed, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("parse double-quoted value: %w", err)
		}
		return parsed, nil
	}

	if value[0] == '\'' {
		if value[len(value)-1] != '\'' {
			return "", errors.New("unterminated single-quoted value")
		}
		return value[1 : len(value)-1], nil
	}

	return value, nil
}
