package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"user-service/internal/service"
)

type Handler struct {
	userSvc *service.UserService
}

func New(userSvc *service.UserService) *Handler {
	return &Handler{userSvc: userSvc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /users/upsert", h.upsert)
	mux.HandleFunc("GET /users/tg/{id}", h.getByTgID)
	mux.HandleFunc("GET /users/role/{role}", h.listByRole)
	mux.HandleFunc("GET /users/all", h.listAll)
	mux.HandleFunc("PUT /users/tg/{id}/role", h.setRole)
	mux.HandleFunc("PUT /users/tg/{id}/phone", h.savePhone)
	mux.HandleFunc("PUT /users/tg/{id}/name", h.updateName)
	mux.HandleFunc("GET /health", h.health)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /users/upsert
func (h *Handler) upsert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TgID      int64  `json:"tg_id"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	user, err := h.userSvc.RegisterOrGet(req.TgID, req.Username, req.FirstName, req.LastName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// GET /users/tg/{id}
func (h *Handler) getByTgID(w http.ResponseWriter, r *http.Request) {
	tgID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	user, err := h.userSvc.GetByTelegramID(tgID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// GET /users/role/{role}
func (h *Handler) listByRole(w http.ResponseWriter, r *http.Request) {
	role := r.PathValue("role")
	users, err := h.userSvc.ListByRole(role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// GET /users/all
func (h *Handler) listAll(w http.ResponseWriter, r *http.Request) {
	users, err := h.userSvc.ListStaff()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// PUT /users/tg/{id}/role
func (h *Handler) setRole(w http.ResponseWriter, r *http.Request) {
	targetTgID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		AdminTgID int64  `json:"admin_tg_id"`
		Role      string `json:"role"`
		BranchID  *int64 `json:"branch_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.userSvc.SetRole(req.AdminTgID, targetTgID, req.Role, req.BranchID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PUT /users/tg/{id}/phone
func (h *Handler) savePhone(w http.ResponseWriter, r *http.Request) {
	tgID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Phone == "" {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.userSvc.SavePhone(tgID, req.Phone); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PUT /users/tg/{id}/name
func (h *Handler) updateName(w http.ResponseWriter, r *http.Request) {
	tgID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		FirstName string `json:"first_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.FirstName == "" {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.userSvc.UpdateName(tgID, req.FirstName); err != nil {
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
