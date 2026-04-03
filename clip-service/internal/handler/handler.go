package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"clip-service/internal/models"
	"clip-service/internal/service"
)

type Handler struct {
	clipSvc *service.ClipService
}

func New(clipSvc *service.ClipService) *Handler {
	return &Handler{clipSvc: clipSvc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /clips", h.createClip)
	mux.HandleFunc("GET /clips/pending", h.listPending)
	mux.HandleFunc("GET /clips/client/{tg_id}", h.listByClient)
	mux.HandleFunc("GET /clips/{id}", h.getClip)
	mux.HandleFunc("GET /files/{id}", h.getClipFile)
	mux.HandleFunc("POST /clips/{id}/payment", h.submitPayment)
	mux.HandleFunc("PUT /clips/{id}/confirm", h.confirmPayment)
	mux.HandleFunc("PUT /clips/{id}/status", h.setStatus)
	mux.HandleFunc("POST /clips/{id}/record", h.recordClip)
	mux.HandleFunc("POST /test/camera", h.testCamera)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) createClip(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ClientTgID int64  `json:"client_tg_id"`
		ClientName string `json:"client_name"`
		BranchID   int64  `json:"branch_id"`
		TableID    int64  `json:"table_id"`
		StartTime  string `json:"start_time"`
		EndTime    string `json:"end_time"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid start_time (RFC3339)")
		return
	}
	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid end_time (RFC3339)")
		return
	}
	cr, err := h.clipSvc.CreateRequest(models.ClipRequestInput{
		ClientTgID: req.ClientTgID,
		ClientName: req.ClientName,
		BranchID:   req.BranchID,
		TableID:    req.TableID,
		StartTime:  startTime,
		EndTime:    endTime,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cr)
}

func (h *Handler) listPending(w http.ResponseWriter, r *http.Request) {
	clips, err := h.clipSvc.ListPending()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, clips)
}

func (h *Handler) listByClient(w http.ResponseWriter, r *http.Request) {
	tgID, err := strconv.ParseInt(r.PathValue("tg_id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tg_id")
		return
	}
	clips, err := h.clipSvc.ListByClient(tgID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, clips)
}

func (h *Handler) getClip(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	cr, err := h.clipSvc.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, cr)
}

// GET /clips/{id}/file — klip MP4 faylini yuklash
func (h *Handler) getClipFile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	data, path, err := h.clipSvc.GetClipFile(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	filename := filepath.Base(path)
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *Handler) submitPayment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		ScreenshotFileID string `json:"screenshot_file_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.clipSvc.CreatePayment(id, req.ScreenshotFileID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) confirmPayment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		AdminID int64 `json:"admin_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.clipSvc.ConfirmPayment(req.AdminID, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		AdminID int64  `json:"admin_id"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.clipSvc.SetStatus(req.AdminID, id, req.Status); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /clips/{id}/record — NVR dan klip yozishni boshlaydi (async)
func (h *Handler) recordClip(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.clipSvc.RecordClip(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "recording"})
}

// POST /test/camera — NVR kamerasini test qiladi (screenshot oladi)
func (h *Handler) testCamera(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BranchID int64 `json:"branch_id"`
		Channel  int   `json:"channel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	path, err := h.clipSvc.TestCamera(req.BranchID, req.Channel)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"screenshot": path})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
