package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

const (
	defaultAddress = ":8080"
	defaultDataDir = "./data"
)

// Config contains only runtime settings. Secret values must never be logged or returned to clients.
type Config struct {
	Address                   string
	DataDir                   string
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

	return Config{
		Address:                   valueOrDefault(lookup("ATOMS_ADDR"), defaultAddress),
		DataDir:                   valueOrDefault(lookup("ATOMS_DATA_DIR"), defaultDataDir),
		AllowPrivateModelEndpoint: parseBool(lookup("ATOMS_ALLOW_PRIVATE_MODEL_ENDPOINTS")),
	}, nil
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
