package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"table-service/internal/service"
)

type Handler struct {
	tableSvc *service.TableService
}

func New(tableSvc *service.TableService) *Handler {
	return &Handler{tableSvc: tableSvc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /branches", h.getBranches)
	mux.HandleFunc("GET /branches/{id}", h.getBranch)
	mux.HandleFunc("GET /branches/{id}/tables", h.getBranchTables)
	mux.HandleFunc("GET /tables/{id}", h.getTable)
	mux.HandleFunc("POST /sessions", h.startSession)
	mux.HandleFunc("PUT /tables/{id}/end", h.endSession)
	mux.HandleFunc("GET /tables/{id}/session", h.activeSession)
	mux.HandleFunc("GET /branches/{id}/report/daily", h.dailyReport)
	mux.HandleFunc("GET /branches/{id}/report/monthly", h.monthlyReport)
	mux.HandleFunc("PUT /branches/{id}/nvr", h.updateNVR)
	mux.HandleFunc("PUT /tables/{id}/rtsp", h.setTableRTSP)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) getBranches(w http.ResponseWriter, r *http.Request) {
	branches, err := h.tableSvc.GetBranches()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, branches)
}

func (h *Handler) getBranch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	branch, err := h.tableSvc.GetBranchByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, branch)
}

func (h *Handler) getBranchTables(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	tables, err := h.tableSvc.GetBranchTables(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tables)
}

func (h *Handler) getTable(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	table, err := h.tableSvc.GetTable(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, table)
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TableID          int64  `json:"table_id"`
		ClientName       string `json:"client_name"`
		OperatorID       int64  `json:"operator_id"`
		OperatorRole     string `json:"operator_role"`
		OperatorBranchID *int64 `json:"operator_branch_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	session, err := h.tableSvc.StartSession(
		req.OperatorID, req.OperatorRole, req.OperatorBranchID,
		req.TableID, req.ClientName,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) endSession(w http.ResponseWriter, r *http.Request) {
	tableID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		OperatorID       int64  `json:"operator_id"`
		OperatorRole     string `json:"operator_role"`
		OperatorBranchID *int64 `json:"operator_branch_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	session, err := h.tableSvc.EndSession(
		req.OperatorID, req.OperatorRole, req.OperatorBranchID, tableID,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) activeSession(w http.ResponseWriter, r *http.Request) {
	tableID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	session, err := h.tableSvc.ActiveSession(tableID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) dailyReport(w http.ResponseWriter, r *http.Request) {
	branchID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	total, priceSom, err := h.tableSvc.DailyReport(branchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": total, "price_som": priceSom})
}

func (h *Handler) monthlyReport(w http.ResponseWriter, r *http.Request) {
	branchID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	total, priceSom, err := h.tableSvc.MonthlyReport(branchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": total, "price_som": priceSom})
}

// PUT /branches/{id}/nvr — NVR sozlamalarini yangilaydi
func (h *Handler) updateNVR(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		IP   string `json:"ip"`
		Port int    `json:"port"`
		User string `json:"user"`
		Pass string `json:"pass"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.tableSvc.UpdateBranchNVR(id, req.IP, req.Port, req.User, req.Pass); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PUT /tables/{id}/rtsp — stol RTSP URL sini yangilaydi
func (h *Handler) setTableRTSP(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		RTSPUrl string `json:"rtsp_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.tableSvc.SetTableRTSP(id, req.RTSPUrl); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
