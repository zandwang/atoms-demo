package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/zand/atoms-demo/internal/config"
)

const maxJSONBodySize = 16 << 10

type apiError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type apiResponse struct {
	Data  any       `json:"data"`
	Error *apiError `json:"error"`
}

type healthData struct {
	Status string             `json:"status"`
	Model  config.ModelStatus `json:"model"`
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	return decodeJSONWithLimit(w, r, destination, maxJSONBodySize)
}

func decodeJSONWithLimit(w http.ResponseWriter, r *http.Request, destination any, maximumSize int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maximumSize)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func writeError(w http.ResponseWriter, status int, code, message string, retryable bool) {
	writeJSON(w, status, apiResponse{
		Data: nil,
		Error: &apiError{
			Code:      code,
			Message:   message,
			Retryable: retryable,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, response apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}
