package httpapi

import (
	"net/http"

	"github.com/sto/crm/backend/internal/service"
)

// handleAdminDashboard — GET /api/admin/dashboard: сводка по загруженности всех филиалов.
func (s *Server) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	data, err := s.admin.Dashboard(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// createBranchRequest — тело запроса создания филиала.
type createBranchRequest struct {
	Address string `json:"address"`
}

// handleAdminCreateBranch — POST /api/admin/branches: создание филиала.
func (s *Server) handleAdminCreateBranch(w http.ResponseWriter, r *http.Request) {
	var req createBranchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}

	branch, err := s.admin.CreateBranch(r.Context(), req.Address)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, branch)
}

// handleAdminCreateManager — POST /api/admin/managers: создание менеджера с привязкой к филиалу.
func (s *Server) handleAdminCreateManager(w http.ResponseWriter, r *http.Request) {
	var req service.CreateManagerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}

	user, err := s.admin.CreateManager(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}
