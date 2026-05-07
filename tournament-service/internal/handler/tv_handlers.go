package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// POST /tv/token  {"tournament_id": N}
func (h *Handler) tvGenerateToken(w http.ResponseWriter, r *http.Request) {
	if !h.checkInternal(w, r) {
		return
	}
	var req struct {
		TournamentID int64 `json:"tournament_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TournamentID == 0 {
		writeError(w, 400, "tournament_id required")
		return
	}
	token, err := h.svc.GenerateTVToken(req.TournamentID)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"token": token})
}

// GET /tv/{token}  — HTML sahifa (TOKEN o'rniga haqiqiy token qo'yiladi)
func (h *Handler) tvPage(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if _, err := h.svc.GetTVData(token); err != nil {
		http.Error(w, "Token topilmadi", http.StatusNotFound)
		return
	}
	html := strings.ReplaceAll(tvHTML, "{{TOKEN}}", token)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	fmt.Fprint(w, html)
}

// GET /tv/{token}/data  — JSON bracket ma'lumot
func (h *Handler) tvData(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	data, err := h.svc.GetTVData(token)
	if err != nil {
		writeError(w, 404, "token topilmadi")
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	writeJSON(w, 200, data)
}

// GET /tv/{token}/sse  — Server-Sent Events stream
func (h *Handler) tvSSE(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if _, err := h.svc.GetTVData(token); err != nil {
		writeError(w, 404, "token topilmadi")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no") // nginx buferini o'chiradi

	// Dastlabki ulanish xabari
	fmt.Fprintf(w, "data: {\"type\":\"connected\"}\n\n")
	flusher.Flush()

	ch := h.hub.subscribe(token)
	defer h.hub.unsubscribe(token, ch)

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprintf(w, "data: {\"type\":\"refresh\"}\n\n")
			flusher.Flush()
		case <-ticker.C:
			// keepalive — nginx/proxy timeout bo'lmasin
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// POST /tv/{token}/push  — barcha ulangan TVlarga refresh yuboradi
func (h *Handler) tvPush(w http.ResponseWriter, r *http.Request) {
	if !h.checkInternal(w, r) {
		return
	}
	token := r.PathValue("token")
	if _, err := h.svc.GetTVData(token); err != nil {
		writeError(w, 404, "token topilmadi")
		return
	}
	h.hub.Broadcast(token)
	writeJSON(w, 200, map[string]any{
		"ok":      true,
		"viewers": h.hub.Count(token),
	})
}

// GET /tv/{token}/status  — admin paneli uchun holat
func (h *Handler) tvStatus(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	data, err := h.svc.GetTVData(token)
	if err != nil {
		writeError(w, 404, "token topilmadi")
		return
	}
	writeJSON(w, 200, map[string]any{
		"token":           token,
		"tournament_id":   data.Tournament.ID,
		"tournament_name": data.Tournament.Name,
		"status":          data.Tournament.Status,
		"viewers":         h.hub.Count(token),
		"matches":         len(data.Matches),
	})
}

// GET /tournaments/{id}/sse  — public SSE, token shart emas
func (h *Handler) tournamentSSE(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, 500, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	fmt.Fprintf(w, "data: {\"type\":\"connected\"}\n\n")
	flusher.Flush()

	key := fmt.Sprintf("trn:%d", id)
	ch := h.hub.subscribe(key)
	defer h.hub.unsubscribe(key, ch)

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprintf(w, "data: {\"type\":\"refresh\"}\n\n")
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// GET /tv/by-tournament?tournament_id=N  — bot-gateway uchun
func (h *Handler) tvGetTokenByTournament(w http.ResponseWriter, r *http.Request) {
	if !h.checkInternal(w, r) {
		return
	}
	idStr := r.URL.Query().Get("tournament_id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id == 0 {
		writeError(w, 400, "tournament_id required")
		return
	}
	token, err := h.svc.GenerateTVToken(id) // idempotent — mavjud bo'lsa qaytaradi
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"token": token})
}
