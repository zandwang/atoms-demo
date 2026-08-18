package app

import (
	"encoding/json"
	"net/http"

	"github.com/zand/atoms-demo/internal/domain"
)

func (s server) handleListMessages(w http.ResponseWriter, r *http.Request, workspace domain.Workspace) {
	messages, err := s.repository.ListMessages(r.Context(), workspace.ID, r.PathValue("projectID"), 100)
	if s.writeProjectError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Data: messages, Error: nil})
}

type previewStateRequest struct {
	State json.RawMessage `json:"state"`
}

func (s server) handleActivateVersion(w http.ResponseWriter, r *http.Request, workspace domain.Workspace) {
	project, err := s.repository.ActivateVersion(r.Context(), workspace.ID, r.PathValue("projectID"), r.PathValue("versionID"))
	if s.writeProjectError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Data: project, Error: nil})
}

func (s server) handleGetPreviewState(w http.ResponseWriter, r *http.Request, workspace domain.Workspace) {
	state, err := s.repository.PreviewState(r.Context(), workspace.ID, r.PathValue("projectID"), r.PathValue("versionID"))
	if s.writeProjectError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Data: state, Error: nil})
}

func (s server) handleSavePreviewState(w http.ResponseWriter, r *http.Request, workspace domain.Workspace) {
	var request previewStateRequest
	if err := decodeJSONWithLimit(w, r, &request, 64<<10); err != nil {
		writeError(w, http.StatusBadRequest, "PREVIEW_STATE_INVALID", "预览返回的数据无效，请刷新预览后重试。", true)
		return
	}
	if err := s.repository.SavePreviewState(r.Context(), workspace.ID, r.PathValue("projectID"), r.PathValue("versionID"), request.State); s.writeProjectError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Data: map[string]bool{"saved": true}, Error: nil})
}

func (s server) handleListVersions(w http.ResponseWriter, r *http.Request, workspace domain.Workspace) {
	versions, err := s.repository.ListVersions(r.Context(), workspace.ID, r.PathValue("projectID"))
	if s.writeProjectError(w, err) {
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Data: versions, Error: nil})
}
