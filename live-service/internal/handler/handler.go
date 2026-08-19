package handler

import (
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"live-service/internal/models"
	"live-service/internal/repository"
	"live-service/internal/streamer"
)

//go:embed static/live.html
var liveHTML string

var errNoCamera = errors.New("stol uchun kamera manzili sozlanmagan")

type Handler struct {
	branchRepo    *repository.BranchRepo
	tableRepo     *repository.TableRepo
	mgr           *streamer.Manager
	internalToken string
	baseURL       string
}

func New(branchRepo *repository.BranchRepo, tableRepo *repository.TableRepo, mgr *streamer.Manager, internalToken, baseURL string) *Handler {
	return &Handler{branchRepo: branchRepo, tableRepo: tableRepo, mgr: mgr, internalToken: internalToken, baseURL: baseURL}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)

	// Internal (bot-gateway) — X-Internal-Token bilan himoyalangan
	mux.HandleFunc("POST /streams/{table_id}/start", h.internal(h.startStream))
	mux.HandleFunc("POST /streams/{table_id}/stop", h.internal(h.stopStream))
	mux.HandleFunc("GET /streams", h.internal(h.listStreams))

	// Public — kirish shart emas (tomoshabinlar uchun)
	mux.HandleFunc("GET /hls/{table_id}/{file...}", h.serveHLS)
	mux.HandleFunc("GET /live/{branch_id}/active", h.activeForBranch)
	mux.HandleFunc("GET /live/{branch_id}", h.livePage)
}

func (h *Handler) internal(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.internalToken != "" && r.Header.Get("X-Internal-Token") != h.internalToken {
			writeErr(w, 401, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) startStream(w http.ResponseWriter, r *http.Request) {
	tableID, err := strconv.ParseInt(r.PathValue("table_id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	table, err := h.tableRepo.GetByID(tableID)
	if err != nil {
		writeErr(w, 404, "stol topilmadi")
		return
	}
	branch, err := h.branchRepo.GetByID(table.BranchID)
	if err != nil {
		writeErr(w, 404, "filial topilmadi")
		return
	}
	rtspURL, err := resolveRTSPURL(branch, table)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if err := h.mgr.Start(tableID, rtspURL); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"status":    "ok",
		"live_url":  h.baseURL + "/live/" + strconv.FormatInt(table.BranchID, 10),
		"table_num": table.TableNum,
	})
}

func (h *Handler) stopStream(w http.ResponseWriter, r *http.Request) {
	tableID, err := strconv.ParseInt(r.PathValue("table_id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	h.mgr.Stop(tableID)
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) listStreams(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, h.mgr.ActiveTableIDs())
}

func (h *Handler) serveHLS(w http.ResponseWriter, r *http.Request) {
	tableID, err := strconv.ParseInt(r.PathValue("table_id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	file := r.PathValue("file")
	if strings.Contains(file, "..") || file == "" {
		http.NotFound(w, r)
		return
	}
	if !h.mgr.IsRunning(tableID) {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(file, ".m3u8"):
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")
	case strings.HasSuffix(file, ".ts"):
		w.Header().Set("Content-Type", "video/mp2t")
		w.Header().Set("Cache-Control", "public, max-age=30")
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	http.ServeFile(w, r, h.mgr.FilePath(tableID, file))
}

func (h *Handler) activeForBranch(w http.ResponseWriter, r *http.Request) {
	branchID, err := strconv.ParseInt(r.PathValue("branch_id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	tables, err := h.tableRepo.ListByBranch(branchID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	type item struct {
		TableID  int64 `json:"table_id"`
		TableNum int   `json:"table_num"`
	}
	list := []item{}
	for _, t := range tables {
		if h.mgr.IsRunning(t.ID) {
			list = append(list, item{TableID: t.ID, TableNum: t.TableNum})
		}
	}
	writeJSON(w, 200, list)
}

func (h *Handler) livePage(w http.ResponseWriter, r *http.Request) {
	branchID, err := strconv.ParseInt(r.PathValue("branch_id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	branch, err := h.branchRepo.GetByID(branchID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	page := strings.ReplaceAll(liveHTML, "{{BRANCH_ID}}", strconv.FormatInt(branchID, 10))
	page = strings.ReplaceAll(page, "{{BRANCH_NAME}}", branch.Name)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(page))
}

// resolveRTSPURL — NVR+kanal (Hikvision) yo'lini afzal ko'radi, bo'lmasa
// stolning o'z RTSP URL'iga tushadi (clip-service bilan bir xil mantiq).
func resolveRTSPURL(branch *models.Branch, table *models.Table) (string, error) {
	if branch.NVRHost != "" && table.CameraChannel > 0 {
		port := branch.NVRPort
		if port == 0 {
			port = 554
		}
		return "rtsp://" + branch.NVRUser + ":" + branch.NVRPass + "@" + branch.NVRHost + ":" +
			strconv.Itoa(port) + "/Streaming/Channels/" + strconv.Itoa(table.CameraChannel) + "01", nil
	}
	if table.RTSPUrl != "" {
		return table.RTSPUrl, nil
	}
	return "", errNoCamera
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
