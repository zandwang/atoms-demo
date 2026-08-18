package app

import (
	"bytes"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/zand/atoms-demo/internal/agent"
	"github.com/zand/atoms-demo/internal/config"
	"github.com/zand/atoms-demo/internal/store"
	"github.com/zand/atoms-demo/internal/webembed"
)

const fallbackIndex = `<!doctype html>
<html lang="zh-CN">
  <head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Atoms Demo</title></head>
  <body><main><h1>Atoms Demo</h1><p>前端资源尚未构建。请运行 <code>make build</code>。</p></main></body>
</html>`

type server struct {
	config     config.Config
	logger     *slog.Logger
	repository store.Repository
	model      agent.ModelAdapter
	gate       *generationGate
	web        fs.FS
}

// NewHandler builds the HTTP surface for local sessions, projects, and later generation work.
func NewHandler(cfg config.Config, logger *slog.Logger, repository store.Repository) http.Handler {
	return NewHandlerWithModel(cfg, logger, repository, nil)
}

// NewHandlerWithModel is the composition point used by tests to supply an
// explicit fake adapter. Production code should use NewHandler.
func NewHandlerWithModel(cfg config.Config, logger *slog.Logger, repository store.Repository, model agent.ModelAdapter) http.Handler {
	s := server{
		config:     cfg,
		logger:     logger,
		repository: repository,
		model:      model,
		gate:       newGenerationGate(),
		web:        webembed.FileSystem(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/session", s.handleSession)
	mux.HandleFunc("POST /api/session/initialize", s.handleInitializeSession)
	mux.HandleFunc("GET /api/projects", s.withWorkspace(s.handleListProjects))
	mux.HandleFunc("POST /api/projects", s.withWorkspace(s.handleCreateProject))
	mux.HandleFunc("GET /api/projects/{projectID}", s.withWorkspace(s.handleGetProject))
	mux.HandleFunc("PATCH /api/projects/{projectID}", s.withWorkspace(s.handleRenameProject))
	mux.HandleFunc("DELETE /api/projects/{projectID}", s.withWorkspace(s.handleDeleteProject))
	mux.HandleFunc("GET /api/projects/{projectID}/messages", s.withWorkspace(s.handleListMessages))
	mux.HandleFunc("GET /api/projects/{projectID}/versions", s.withWorkspace(s.handleListVersions))
	mux.HandleFunc("POST /api/projects/{projectID}/versions/{versionID}/activate", s.withWorkspace(s.handleActivateVersion))
	mux.HandleFunc("GET /api/projects/{projectID}/versions/{versionID}/preview-state", s.withWorkspace(s.handleGetPreviewState))
	mux.HandleFunc("PUT /api/projects/{projectID}/versions/{versionID}/preview-state", s.withWorkspace(s.handleSavePreviewState))
	mux.HandleFunc("POST /api/projects/{projectID}/generate", s.withWorkspace(s.handleGenerate))
	mux.HandleFunc("GET /api/{path...}", s.handleAPINotFound)
	mux.HandleFunc("POST /api/{path...}", s.handleAPINotFound)
	mux.HandleFunc("PUT /api/{path...}", s.handleAPINotFound)
	mux.HandleFunc("PATCH /api/{path...}", s.handleAPINotFound)
	mux.HandleFunc("DELETE /api/{path...}", s.handleAPINotFound)
	mux.HandleFunc("GET /{$}", s.serveIndex)
	mux.HandleFunc("GET /{path...}", s.serveApplication)

	return withRequestLogging(logger, withSecurityHeaders(mux))
}

func (s server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, apiResponse{
		Data: healthData{
			Status: "ok",
			Model:  s.config.ModelStatus(),
		},
		Error: nil,
	})
}

func (s server) handleAPINotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "NOT_FOUND", "未找到 API 路由。", false)
}

func (s server) serveApplication(w http.ResponseWriter, r *http.Request) {
	candidate := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if candidate != "" && candidate != "." && fs.ValidPath(candidate) && s.serveFile(w, r, candidate) {
		return
	}
	s.serveIndex(w, r)
}

func (s server) serveFile(w http.ResponseWriter, r *http.Request, filename string) bool {
	contents, err := fs.ReadFile(s.web, filename)
	if err != nil {
		return false
	}

	if contentType := mime.TypeByExtension(path.Ext(filename)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, path.Base(filename), time.Time{}, bytes.NewReader(contents))
	return true
}

func (s server) serveIndex(w http.ResponseWriter, r *http.Request) {
	if s.serveFile(w, r, "index.html") {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(fallbackIndex))
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func withRequestLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(startedAt))
	})
}
