package identity

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Service) handleTVLogin(w http.ResponseWriter, r *http.Request) {
	var in TVLoginInput
	if err := decodeJSON(w, r, &in); err != nil {
		writeError(w, r, err)
		return
	}
	result, err := s.TVLogin(r.Context(), in)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Service) handleListTVs(w http.ResponseWriter, r *http.Request) {
	items, err := s.ListTVs(r.Context(), AccountIDFrom(r.Context()))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Service) handleGetTV(w http.ResponseWriter, r *http.Request) {
	item, err := s.TVByID(r.Context(), AccountIDFrom(r.Context()), chi.URLParam(r, "deviceID"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Service) handleAssignTVChild(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Nickname string `json:"nickname"`
		Grade    string `json:"grade_tag"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, err)
		return
	}
	item, err := s.AssignTVChild(r.Context(), AccountIDFrom(r.Context()),
		chi.URLParam(r, "deviceID"), req.Nickname, req.Grade)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Service) handleDeleteTV(w http.ResponseWriter, r *http.Request) {
	if err := s.DeleteTV(r.Context(), AccountIDFrom(r.Context()), chi.URLParam(r, "deviceID")); err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Service) handleTVMe(w http.ResponseWriter, r *http.Request) {
	principal, ok := DeviceFrom(r.Context())
	if !ok {
		writeError(w, r, ErrInvalidToken)
		return
	}
	if principal.Kind != "tv" {
		writeJSON(w, http.StatusOK, map[string]any{"kind": "child", "device_id": principal.DeviceID})
		return
	}
	item, err := s.TVMe(r.Context(), principal)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
