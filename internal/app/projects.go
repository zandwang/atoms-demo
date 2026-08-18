package app

import (
	"errors"
	"net/http"

	"github.com/zand/atoms-demo/internal/domain"
	"github.com/zand/atoms-demo/internal/store"
)

type projectRequest struct {
	Name string `json:"name"`
}

func (s server) handleListProjects(w http.ResponseWriter, r *http.Request, workspace domain.Workspace) {
	projects, err := s.repository.ListProjects(r.Context(), workspace.ID)
	if err != nil {
		s.logger.Error("list projects", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "无法读取项目。", true)
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Data: projects, Error: nil})
}

func (s server) handleGetProject(w http.ResponseWriter, r *http.Request, workspace domain.Workspace) {
	project, err := s.repository.ProjectByID(r.Context(), workspace.ID, r.PathValue("projectID"))
	if s.writeProjectError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Data: project, Error: nil})
}

func (s server) handleCreateProject(w http.ResponseWriter, r *http.Request, workspace domain.Workspace) {
	var request projectRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请输入有效的项目名称。", false)
		return
	}
	project, err := s.repository.CreateProject(r.Context(), workspace.ID, request.Name)
	if s.writeProjectError(w, err) {
		return
	}
	writeJSON(w, http.StatusCreated, apiResponse{Data: project, Error: nil})
}

func (s server) handleRenameProject(w http.ResponseWriter, r *http.Request, workspace domain.Workspace) {
	var request projectRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "请输入有效的项目名称。", false)
		return
	}
	project, err := s.repository.RenameProject(r.Context(), workspace.ID, r.PathValue("projectID"), request.Name)
	if s.writeProjectError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Data: project, Error: nil})
}

func (s server) handleDeleteProject(w http.ResponseWriter, r *http.Request, workspace domain.Workspace) {
	err := s.repository.DeleteProject(r.Context(), workspace.ID, r.PathValue("projectID"))
	if s.writeProjectError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Data: map[string]bool{"deleted": true}, Error: nil})
}

func (s server) writeProjectError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, domain.ErrInvalidProjectName) {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "项目名称长度需为 1–100 个字符。", false)
		return true
	}
	if errors.Is(err, store.ErrProjectNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "未找到该项目。", false)
		return true
	}
	if errors.Is(err, domain.ErrInvalidPreviewState) {
		writeError(w, http.StatusBadRequest, "PREVIEW_STATE_INVALID", "预览返回的数据无效，请刷新预览后重试。", true)
		return true
	}
	s.logger.Error("project operation", "error", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "无法更新项目。", true)
	return true
}
