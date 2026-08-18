package app

import (
	"errors"
	"net/http"
	"time"

	"github.com/zand/atoms-demo/internal/domain"
	"github.com/zand/atoms-demo/internal/store"
)

const (
	sessionCookieName   = "atoms_demo_session"
	sessionCookieMaxAge = 365 * 24 * 60 * 60
)

type sessionData struct {
	Initialized bool              `json:"initialized"`
	Workspace   *domain.Workspace `json:"workspace"`
}

type initializeSessionRequest struct {
	DisplayName string `json:"displayName"`
}

type workspaceHandler func(http.ResponseWriter, *http.Request, domain.Workspace)

func (s server) withWorkspace(next workspaceHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspace, err := s.workspaceFromRequest(r)
		if errors.Is(err, store.ErrSessionNotFound) {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先初始化本地工作区。", false)
			return
		}
		if err != nil {
			s.logger.Error("load workspace from session", "error", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "无法读取本地工作区。", true)
			return
		}
		next(w, r, workspace)
	}
}

func (s server) handleSession(w http.ResponseWriter, r *http.Request) {
	workspace, err := s.workspaceFromRequest(r)
	if errors.Is(err, store.ErrSessionNotFound) {
		writeJSON(w, http.StatusOK, apiResponse{Data: sessionData{Initialized: false, Workspace: nil}, Error: nil})
		return
	}
	if err != nil {
		s.logger.Error("load session", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "无法读取本地会话。", true)
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Data: sessionData{Initialized: true, Workspace: &workspace}, Error: nil})
}

func (s server) handleInitializeSession(w http.ResponseWriter, r *http.Request) {
	if workspace, err := s.workspaceFromRequest(r); err == nil {
		writeJSON(w, http.StatusOK, apiResponse{Data: sessionData{Initialized: true, Workspace: &workspace}, Error: nil})
		return
	} else if !errors.Is(err, store.ErrSessionNotFound) {
		s.logger.Error("load existing session", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "无法初始化本地工作区。", true)
		return
	}

	var request initializeSessionRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请输入有效的昵称。", false)
		return
	}
	if s.repository == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "本地存储尚未初始化。", true)
		return
	}

	workspace, token, err := s.repository.InitializeWorkspace(r.Context(), request.DisplayName)
	if errors.Is(err, domain.ErrInvalidDisplayName) {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "昵称长度需为 1–80 个字符。", false)
		return
	}
	if err != nil {
		s.logger.Error("initialize workspace", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "无法初始化本地工作区。", true)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   sessionCookieMaxAge,
		Expires:  time.Now().Add(sessionCookieMaxAge * time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
	writeJSON(w, http.StatusCreated, apiResponse{Data: sessionData{Initialized: true, Workspace: &workspace}, Error: nil})
}

func (s server) workspaceFromRequest(r *http.Request) (domain.Workspace, error) {
	if s.repository == nil {
		return domain.Workspace{}, errors.New("repository is not configured")
	}
	cookie, err := r.Cookie(sessionCookieName)
	if errors.Is(err, http.ErrNoCookie) {
		return domain.Workspace{}, store.ErrSessionNotFound
	}
	if err != nil {
		return domain.Workspace{}, err
	}
	return s.repository.WorkspaceBySession(r.Context(), cookie.Value)
}
