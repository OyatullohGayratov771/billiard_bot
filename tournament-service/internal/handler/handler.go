package handler

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"tournament-service/internal/service"
)

//go:embed static/tv_bracket.html
var tvHTML string

type Handler struct {
	svc           *service.Service
	hub           *Hub
	internalToken string
}

func New(svc *service.Service, internalToken string) *Handler {
	return &Handler{svc: svc, hub: newHub(), internalToken: internalToken}
}

// checkInternal returns false and writes 401 if INTERNAL_TOKEN is set and header doesn't match.
func (h *Handler) checkInternal(w http.ResponseWriter, r *http.Request) bool {
	if h.internalToken == "" {
		return true
	}
	if r.Header.Get("X-Internal-Token") != h.internalToken {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	return true
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)

	mux.HandleFunc("POST /tournaments", h.createTournament)
	mux.HandleFunc("GET /tournaments", h.listTournaments)
	mux.HandleFunc("GET /tournaments/{id}", h.getTournament)
	mux.HandleFunc("PUT /tournaments/{id}", h.updateTournament)
	mux.HandleFunc("DELETE /tournaments/{id}", h.deleteTournament)
	mux.HandleFunc("PUT /tournaments/{id}/cancel", h.cancelTournament)

	mux.HandleFunc("POST /tournaments/{id}/register", h.register)
	mux.HandleFunc("POST /tournaments/{id}/register-manual", h.registerManual)
	mux.HandleFunc("GET /tournaments/{id}/registrations", h.listRegistrations)
	mux.HandleFunc("PUT /tournaments/{id}/registrations/{reg_id}/approve", h.approveReg)
	mux.HandleFunc("PUT /tournaments/{id}/registrations/{reg_id}/reject", h.rejectReg)

	mux.HandleFunc("POST /tournaments/{id}/bracket", h.generateBracket)
	mux.HandleFunc("POST /tournaments/{id}/bracket/shuffle", h.shuffleBracket)
	mux.HandleFunc("GET /tournaments/{id}/bracket", h.getBracket)
	mux.HandleFunc("GET /tournaments/{id}/tv-token", h.tvTokenPublic)
	mux.HandleFunc("GET /tournaments/sse", h.tournamentsSSE)
	mux.HandleFunc("GET /tournaments/{id}/sse", h.tournamentSSE)

	mux.HandleFunc("PUT /matches/{id}/table", h.setMatchTable)
	mux.HandleFunc("PUT /matches/{id}/schedule", h.setMatchSchedule)
	mux.HandleFunc("PUT /matches/{id}/start", h.startMatch)
	mux.HandleFunc("PUT /matches/{id}/result", h.setResult)

	mux.HandleFunc("GET /users/{tg_id}/tournaments", h.userTournaments)

	// TV endpoints
	mux.HandleFunc("POST /tv/token", h.tvGenerateToken)
	mux.HandleFunc("GET /tv/by-tournament", h.tvGetTokenByTournament)
	mux.HandleFunc("GET /tv/{token}", h.tvPage)
	mux.HandleFunc("GET /tv/{token}/data", h.tvData)
	mux.HandleFunc("GET /tv/{token}/sse", h.tvSSE)
	mux.HandleFunc("POST /tv/{token}/push", h.tvPush)
	mux.HandleFunc("GET /tv/{token}/status", h.tvStatus)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) createTournament(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		BranchID    int64  `json:"branch_id"`
		TableID     *int64 `json:"table_id"`
		ScheduledAt string `json:"scheduled_at"`
		Price       int64  `json:"price"`
		MaxPlayers  int    `json:"max_players"`
		AdminTgID   int64  `json:"admin_tg_id"`
		JoinCode    string `json:"join_code"`
		Type        string `json:"type"`
		TableCount  int    `json:"table_count"`
		SlotMinutes int    `json:"slot_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	t, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		writeError(w, 400, "invalid scheduled_at (RFC3339)")
		return
	}
	tournament, err := h.svc.CreateTournament(req.Name, req.BranchID, req.TableID, t, req.Price, req.MaxPlayers, req.AdminTgID, req.JoinCode, req.Type, req.TableCount, req.SlotMinutes)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	h.hub.Broadcast("tournaments")
	writeJSON(w, 200, tournament)
}

func (h *Handler) listTournaments(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	list, err := h.svc.ListTournaments(status)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, list)
}

func (h *Handler) getTournament(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	t, err := h.svc.GetTournament(id)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, t)
}

func (h *Handler) updateTournament(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	var req struct {
		Name        string `json:"name"`
		ScheduledAt string `json:"scheduled_at"`
		MaxPlayers  int    `json:"max_players"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	t, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		writeError(w, 400, "invalid scheduled_at (RFC3339)")
		return
	}
	if err := h.svc.UpdateTournament(id, req.Name, t, req.MaxPlayers); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	h.hub.Broadcast("tournaments")
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) cancelTournament(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	if err := h.svc.CancelTournament(id); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	h.hub.Broadcast("tournaments")
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) deleteTournament(w http.ResponseWriter, r *http.Request) {
	if !h.checkInternal(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	if err := h.svc.DeleteTournament(id); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	h.hub.Broadcast("tournaments")
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	var req struct {
		UserTgID int64  `json:"user_tg_id"`
		UserName string `json:"user_name"`
		JoinCode string `json:"join_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	reg, err := h.svc.Register(id, req.UserTgID, req.UserName, req.JoinCode)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, reg)
}

func (h *Handler) registerManual(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	var req struct {
		PlayerName string `json:"player_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	reg, err := h.svc.RegisterManual(id, req.PlayerName)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	h.hub.Broadcast("tournaments")
	writeJSON(w, 200, reg)
}

func (h *Handler) listRegistrations(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	list, err := h.svc.ListRegistrations(id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, list)
}

func (h *Handler) approveReg(w http.ResponseWriter, r *http.Request) {
	tID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	rID, err := strconv.ParseInt(r.PathValue("reg_id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid reg_id")
		return
	}
	if err := h.svc.ApproveRegistration(tID, rID); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	h.hub.Broadcast("tournaments")
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) rejectReg(w http.ResponseWriter, r *http.Request) {
	tID, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	rID, err := strconv.ParseInt(r.PathValue("reg_id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid reg_id")
		return
	}
	if err := h.svc.RejectRegistration(tID, rID); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	h.hub.Broadcast("tournaments")
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) generateBracket(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	matches, err := h.svc.GenerateBracket(id)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	h.hub.Broadcast("tournaments")
	writeJSON(w, 200, matches)
}

func (h *Handler) shuffleBracket(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	matches, err := h.svc.ShuffleBracket(id)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	h.hub.Broadcast(fmt.Sprintf("trn:%d", id))
	writeJSON(w, 200, matches)
}

func (h *Handler) setMatchTable(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	var req struct {
		TableNum int `json:"table_num"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	if req.TableNum <= 0 {
		writeError(w, 400, "stol raqami 0 dan katta bo'lishi kerak")
		return
	}
	if err := h.svc.SetMatchTable(id, req.TableNum); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	match, _ := h.svc.GetMatch(id)
	if match != nil {
		h.hub.Broadcast(fmt.Sprintf("trn:%d", match.TournamentID))
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) setMatchSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	var req struct {
		ScheduledAt string `json:"scheduled_at"` // RFC3339
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	t, err := time.Parse(time.RFC3339, req.ScheduledAt)
	if err != nil {
		writeError(w, 400, "invalid scheduled_at (RFC3339)")
		return
	}
	if err := h.svc.SetMatchScheduledAt(id, t); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	match, _ := h.svc.GetMatch(id)
	if match != nil {
		h.hub.Broadcast(fmt.Sprintf("trn:%d", match.TournamentID))
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) startMatch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	var req struct {
		TableNum int `json:"table_num"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	if err := h.svc.StartMatch(id, req.TableNum); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	match, _ := h.svc.GetMatch(id)
	if match != nil {
		h.hub.Broadcast(fmt.Sprintf("trn:%d", match.TournamentID))
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) getBracket(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	matches, err := h.svc.GetBracket(id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, matches)
}

func (h *Handler) setResult(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid id")
		return
	}
	var req struct {
		WinnerSlot int   `json:"winner_slot"`  // 1 or 2
		WinnerTgID int64 `json:"winner_tg_id"` // legacy, ignored if winner_slot set
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request")
		return
	}
	var winnerTgID int64
	if req.WinnerSlot == 1 || req.WinnerSlot == 2 {
		m, err2 := h.svc.GetMatch(id)
		if err2 != nil {
			writeError(w, 404, "o'yin topilmadi")
			return
		}
		if req.WinnerSlot == 1 {
			if m.Player1TgID == nil {
				writeError(w, 400, "1-o'yinchi belgilanmagan")
				return
			}
			winnerTgID = *m.Player1TgID
		} else {
			if m.Player2TgID == nil {
				writeError(w, 400, "2-o'yinchi belgilanmagan")
				return
			}
			winnerTgID = *m.Player2TgID
		}
	} else {
		winnerTgID = req.WinnerTgID
	}
	winnerNextID, loserNextID, assignedMatchID, assignedTableNum, finished, trnID, err := h.svc.SetResult(id, winnerTgID)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	h.hub.Broadcast(fmt.Sprintf("trn:%d", trnID))
	h.hub.Broadcast("tournaments")
	writeJSON(w, 200, map[string]any{
		"next_match_id":        winnerNextID,
		"loser_next_match_id":  loserNextID,
		"finished":             finished,
		"assigned_match_id":    assignedMatchID,
		"assigned_table_num":   assignedTableNum,
	})
}

func (h *Handler) userTournaments(w http.ResponseWriter, r *http.Request) {
	tgID, err := strconv.ParseInt(r.PathValue("tg_id"), 10, 64)
	if err != nil {
		writeError(w, 400, "invalid tg_id")
		return
	}
	list, err := h.svc.GetUserTournaments(tgID)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, list)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
