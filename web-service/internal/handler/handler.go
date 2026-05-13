package handler

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type Handler struct {
	db      *sql.DB
	secret  string
	baseURL string
}

func New(db *sql.DB, secret, baseURL string) *Handler {
	return &Handler{db: db, secret: secret, baseURL: baseURL}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("POST /internal/token", h.createToken)
	mux.HandleFunc("GET /api/auth/session", h.session)
	mux.HandleFunc("GET /api/me", h.withAuth(h.me))
	mux.HandleFunc("GET /api/me/clips", h.withAuth(h.myClips))
	mux.HandleFunc("GET /api/me/tournaments", h.withAuth(h.myTournaments))
	mux.HandleFunc("GET /api/admin/stats", h.withRole(h.adminStats, "admin", "superadmin"))
	mux.HandleFunc("GET /api/admin/clips", h.withRole(h.adminClips, "admin", "superadmin"))
	mux.HandleFunc("GET /api/admin/tournaments", h.withRole(h.adminTournaments, "admin", "superadmin"))
	mux.HandleFunc("GET /api/admin/users", h.withRole(h.adminUsers, "admin", "superadmin", "operator"))
}

// ─── JWT (HMAC-SHA256, no external library) ───

type Claims struct {
	TgID int64  `json:"tg_id"`
	Role string `json:"role"`
	Exp  int64  `json:"exp"`
}

type ctxKey struct{}

func (h *Handler) signJWT(c Claims) string {
	hdr := b64u([]byte(`{"alg":"HS256","typ":"JWT"}`))
	pay, _ := json.Marshal(c)
	body := hdr + "." + b64u(pay)
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write([]byte(body))
	return body + "." + b64u(mac.Sum(nil))
}

func (h *Handler) verifyJWT(token string) (*Claims, bool) {
	p := strings.Split(token, ".")
	if len(p) != 3 {
		return nil, false
	}
	body := p[0] + "." + p[1]
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write([]byte(body))
	if b64u(mac.Sum(nil)) != p[2] {
		return nil, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(p[1])
	if err != nil {
		return nil, false
	}
	var c Claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, false
	}
	if time.Now().Unix() > c.Exp {
		return nil, false
	}
	return &c, true
}

func b64u(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

// ─── Middleware ───

func (h *Handler) extractClaims(r *http.Request) *Claims {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		if c, ok := h.verifyJWT(strings.TrimPrefix(auth, "Bearer ")); ok {
			return c
		}
	}
	if cookie, err := r.Cookie("bk_jwt"); err == nil {
		if c, ok := h.verifyJWT(cookie.Value); ok {
			return c
		}
	}
	return nil
}

func (h *Handler) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c := h.extractClaims(r)
		if c == nil {
			writeErr(w, 401, "unauthorized")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, c)))
	}
}

func (h *Handler) withRole(next http.HandlerFunc, roles ...string) http.HandlerFunc {
	return h.withAuth(func(w http.ResponseWriter, r *http.Request) {
		c := r.Context().Value(ctxKey{}).(*Claims)
		for _, role := range roles {
			if c.Role == role {
				next(w, r)
				return
			}
		}
		writeErr(w, 403, "forbidden")
	})
}

func getClaims(r *http.Request) *Claims {
	return r.Context().Value(ctxKey{}).(*Claims)
}

// ─── Handlers ───

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) createToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TgID int64  `json:"tg_id"`
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TgID == 0 {
		writeErr(w, 400, "invalid request")
		return
	}
	if req.Role == "" {
		req.Role = "client"
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)

	_, _ = h.db.Exec(`DELETE FROM web_tokens WHERE expires_at < NOW()`)
	if _, err := h.db.Exec(
		`INSERT INTO web_tokens(token,tg_id,role,expires_at) VALUES($1,$2,$3,$4)`,
		token, req.TgID, req.Role, time.Now().Add(5*time.Minute),
	); err != nil {
		log.Printf("createToken: %v", err)
		writeErr(w, 500, "db error")
		return
	}
	page := "/me"
	if req.Role == "admin" || req.Role == "superadmin" || req.Role == "operator" {
		page = "/panel"
	}
	writeJSON(w, 200, map[string]string{
		"token": token,
		"url":   h.baseURL + page + "?t=" + token,
	})
}

func (h *Handler) session(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("t")
	if token == "" {
		writeErr(w, 400, "missing token")
		return
	}
	var tgID int64
	var role string
	err := h.db.QueryRow(
		`DELETE FROM web_tokens WHERE token=$1 AND expires_at > NOW() RETURNING tg_id, role`,
		token,
	).Scan(&tgID, &role)
	if err == sql.ErrNoRows {
		writeErr(w, 401, "token expired or invalid")
		return
	}
	if err != nil {
		writeErr(w, 500, "db error")
		return
	}
	jwt := h.signJWT(Claims{TgID: tgID, Role: role, Exp: time.Now().Add(24 * time.Hour).Unix()})
	writeJSON(w, 200, map[string]any{"jwt": jwt, "role": role, "tg_id": tgID})
}

// ─── /api/me ───

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	tgID := getClaims(r).TgID
	var u struct {
		TgID      int64  `json:"tg_id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
		Phone     string `json:"phone"`
		Role      string `json:"role"`
		CreatedAt string `json:"created_at"`
	}
	err := h.db.QueryRow(
		`SELECT tg_id, first_name, COALESCE(last_name,''), COALESCE(username,''),
		        COALESCE(phone,''), role, created_at FROM users WHERE tg_id=$1`, tgID,
	).Scan(&u.TgID, &u.FirstName, &u.LastName, &u.Username, &u.Phone, &u.Role, &u.CreatedAt)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "user not found")
		return
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, u)
}

func (h *Handler) myClips(w http.ResponseWriter, r *http.Request) {
	tgID := getClaims(r).TgID
	rows, err := h.db.Query(`
		SELECT cr.id, COALESCE(b.name,'?'), COALESCE(t.table_num,0),
		       cr.start_time, cr.end_time, cr.status, COALESCE(cr.notes,''), cr.created_at
		FROM clip_requests cr
		LEFT JOIN branches b ON b.id=cr.branch_id
		LEFT JOIN tables t ON t.id=cr.table_id
		WHERE cr.client_tg_id=$1 ORDER BY cr.created_at DESC LIMIT 30`, tgID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type item struct {
		ID         int64  `json:"id"`
		BranchName string `json:"branch_name"`
		TableNum   int    `json:"table_num"`
		StartTime  string `json:"start_time"`
		EndTime    string `json:"end_time"`
		Status     string `json:"status"`
		Notes      string `json:"notes"`
		CreatedAt  string `json:"created_at"`
	}
	list := []item{}
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.ID, &i.BranchName, &i.TableNum,
			&i.StartTime, &i.EndTime, &i.Status, &i.Notes, &i.CreatedAt); err == nil {
			list = append(list, i)
		}
	}
	writeJSON(w, 200, list)
}

func (h *Handler) myTournaments(w http.ResponseWriter, r *http.Request) {
	tgID := getClaims(r).TgID
	rows, err := h.db.Query(`
		SELECT tr.tournament_id, t.name, COALESCE(b.name,'?'),
		       t.scheduled_at, t.status, tr.status, tr.registered_at
		FROM tournament_registrations tr
		JOIN tournaments t ON t.id=tr.tournament_id
		LEFT JOIN branches b ON b.id=t.branch_id
		WHERE tr.user_tg_id=$1 ORDER BY tr.registered_at DESC LIMIT 20`, tgID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type item struct {
		TournamentID     int64  `json:"tournament_id"`
		Name             string `json:"name"`
		BranchName       string `json:"branch_name"`
		ScheduledAt      string `json:"scheduled_at"`
		TournamentStatus string `json:"tournament_status"`
		RegStatus        string `json:"reg_status"`
		RegisteredAt     string `json:"registered_at"`
	}
	list := []item{}
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.TournamentID, &i.Name, &i.BranchName,
			&i.ScheduledAt, &i.TournamentStatus, &i.RegStatus, &i.RegisteredAt); err == nil {
			list = append(list, i)
		}
	}
	writeJSON(w, 200, list)
}

// ─── /api/admin ───

func (h *Handler) adminStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]int{}
	queries := map[string]string{
		"users":              `SELECT COUNT(*) FROM users`,
		"pending_clips":      `SELECT COUNT(*) FROM clip_requests WHERE status='pending'`,
		"paid_clips":         `SELECT COUNT(*) FROM clip_requests WHERE status='paid'`,
		"processing_clips":   `SELECT COUNT(*) FROM clip_requests WHERE status='processing'`,
		"active_tournaments": `SELECT COUNT(*) FROM tournaments WHERE status IN ('registration','in_progress')`,
	}
	for k, q := range queries {
		var n int
		_ = h.db.QueryRow(q).Scan(&n)
		stats[k] = n
	}
	writeJSON(w, 200, stats)
}

func (h *Handler) adminClips(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	q := `SELECT cr.id, cr.client_name, COALESCE(b.name,'?'), COALESCE(t.table_num,0),
		       cr.start_time, cr.end_time, cr.status, COALESCE(cr.notes,''), cr.created_at
		FROM clip_requests cr
		LEFT JOIN branches b ON b.id=cr.branch_id
		LEFT JOIN tables t ON t.id=cr.table_id`
	var args []any
	if status != "" {
		q += " WHERE cr.status=$1"
		args = append(args, status)
	}
	q += " ORDER BY cr.created_at DESC LIMIT 50"
	rows, err := h.db.Query(q, args...)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type item struct {
		ID         int64  `json:"id"`
		ClientName string `json:"client_name"`
		BranchName string `json:"branch_name"`
		TableNum   int    `json:"table_num"`
		StartTime  string `json:"start_time"`
		EndTime    string `json:"end_time"`
		Status     string `json:"status"`
		Notes      string `json:"notes"`
		CreatedAt  string `json:"created_at"`
	}
	list := []item{}
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.ID, &i.ClientName, &i.BranchName, &i.TableNum,
			&i.StartTime, &i.EndTime, &i.Status, &i.Notes, &i.CreatedAt); err == nil {
			list = append(list, i)
		}
	}
	writeJSON(w, 200, list)
}

func (h *Handler) adminTournaments(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT t.id, t.name, COALESCE(b.name,'?'), t.scheduled_at, t.status, t.max_players,
		       COUNT(tr.id) FILTER (WHERE tr.status='approved')
		FROM tournaments t
		LEFT JOIN branches b ON b.id=t.branch_id
		LEFT JOIN tournament_registrations tr ON tr.tournament_id=t.id
		GROUP BY t.id, b.name ORDER BY t.created_at DESC LIMIT 30`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type item struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		BranchName  string `json:"branch_name"`
		ScheduledAt string `json:"scheduled_at"`
		Status      string `json:"status"`
		MaxPlayers  int    `json:"max_players"`
		Registered  int    `json:"registered"`
	}
	list := []item{}
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.ID, &i.Name, &i.BranchName, &i.ScheduledAt,
			&i.Status, &i.MaxPlayers, &i.Registered); err == nil {
			list = append(list, i)
		}
	}
	writeJSON(w, 200, list)
}

func (h *Handler) adminUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT tg_id, first_name, COALESCE(username,''), role, created_at
		FROM users ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type item struct {
		TgID      int64  `json:"tg_id"`
		FirstName string `json:"first_name"`
		Username  string `json:"username"`
		Role      string `json:"role"`
		CreatedAt string `json:"created_at"`
	}
	list := []item{}
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.TgID, &i.FirstName, &i.Username, &i.Role, &i.CreatedAt); err == nil {
			list = append(list, i)
		}
	}
	writeJSON(w, 200, list)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
