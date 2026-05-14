package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	db        *sql.DB
	secret    string
	baseURL   string
	botToken  string
	trnSvcURL string
}

func New(db *sql.DB, secret, baseURL, botToken, trnSvcURL string) *Handler {
	return &Handler{db: db, secret: secret, baseURL: baseURL, botToken: botToken, trnSvcURL: trnSvcURL}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)

	// Auth
	mux.HandleFunc("POST /api/auth/tg", h.authTelegram)

	// Client routes
	mux.HandleFunc("GET /api/me", h.withAuth(h.me))
	mux.HandleFunc("GET /api/me/clips", h.withAuth(h.myClips))
	mux.HandleFunc("GET /api/me/tournaments", h.withAuth(h.myTournaments))

	// Admin routes (operator, admin, superadmin)
	mux.HandleFunc("GET /api/admin/stats", h.withRole(h.adminStats, "operator", "admin", "superadmin"))
	mux.HandleFunc("GET /api/admin/clips", h.withRole(h.adminClips, "operator", "admin", "superadmin"))
	mux.HandleFunc("GET /api/admin/tournaments", h.withRole(h.adminTournaments, "operator", "admin", "superadmin"))
	mux.HandleFunc("GET /api/admin/users", h.withRole(h.adminUsers, "admin", "superadmin"))

	// Superadmin-only user management
	mux.HandleFunc("PUT /api/admin/users/{tgid}/role", h.withRole(h.adminSetRole, "superadmin"))
	mux.HandleFunc("PUT /api/admin/users/{tgid}/active", h.withRole(h.adminSetActive, "superadmin"))

	// Tournament registration proxy (auth via JWT, proxies to tournament-service)
	mux.HandleFunc("POST /api/me/tournaments/{id}/register", h.withAuth(h.meTournamentRegister))

	// Profile update
	mux.HandleFunc("PUT /api/me/profile", h.withAuth(h.meUpdateProfile))
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

// ─── Telegram initData verification ───

type tgWebUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
}

func (h *Handler) verifyInitData(initData string) (tgWebUser, error) {
	params, err := url.ParseQuery(initData)
	if err != nil {
		return tgWebUser{}, err
	}
	hash := params.Get("hash")
	if hash == "" {
		return tgWebUser{}, errors.New("no hash")
	}
	params.Del("hash")

	var parts []string
	for k, v := range params {
		if len(v) > 0 {
			parts = append(parts, k+"="+v[0])
		}
	}
	sort.Strings(parts)
	dataCheck := strings.Join(parts, "\n")

	// secret_key = HMAC-SHA256(key="WebAppData", data=botToken)
	mac1 := hmac.New(sha256.New, []byte("WebAppData"))
	mac1.Write([]byte(h.botToken))
	secretKey := mac1.Sum(nil)

	// sig = hex(HMAC-SHA256(key=secretKey, data=dataCheck))
	mac2 := hmac.New(sha256.New, secretKey)
	mac2.Write([]byte(dataCheck))
	sig := hex.EncodeToString(mac2.Sum(nil))

	if sig != hash {
		return tgWebUser{}, errors.New("hash mismatch")
	}

	// Reject tokens older than 24 hours
	if authDate, _ := strconv.ParseInt(params.Get("auth_date"), 10, 64); authDate > 0 {
		if time.Now().Unix()-authDate > 86400 {
			return tgWebUser{}, errors.New("initData too old")
		}
	}

	var user tgWebUser
	if err := json.Unmarshal([]byte(params.Get("user")), &user); err != nil || user.ID == 0 {
		return tgWebUser{}, errors.New("invalid user")
	}
	return user, nil
}

// ─── Middleware ───

func (h *Handler) extractClaims(r *http.Request) *Claims {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		if c, ok := h.verifyJWT(strings.TrimPrefix(auth, "Bearer ")); ok {
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

// ─── Auth ───

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) authTelegram(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InitData string `json:"init_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.InitData == "" {
		writeErr(w, 400, "missing init_data")
		return
	}

	tgUser, err := h.verifyInitData(req.InitData)
	if err != nil {
		writeErr(w, 401, err.Error())
		return
	}

	// Auto-register new users; update username for existing users.
	// ON CONFLICT keeps is_active and role unchanged for existing rows.
	var role string
	var isActive bool
	err = h.db.QueryRow(`
		INSERT INTO users (telegram_id, username, first_name, last_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (telegram_id) DO UPDATE
		  SET username   = EXCLUDED.username,
		      first_name = COALESCE(NULLIF(EXCLUDED.first_name, ''), users.first_name),
		      last_name  = COALESCE(NULLIF(EXCLUDED.last_name,  ''), users.last_name)
		RETURNING role, is_active
	`, tgUser.ID, tgUser.Username, tgUser.FirstName, tgUser.LastName).Scan(&role, &isActive)
	if err != nil {
		// Fallback: try plain select
		_ = h.db.QueryRow(`SELECT role, is_active FROM users WHERE telegram_id=$1`,
			tgUser.ID).Scan(&role, &isActive)
		if role == "" {
			role = "client"
			isActive = true
		}
	}

	if !isActive {
		writeErr(w, 403, "account blocked")
		return
	}

	jwt := h.signJWT(Claims{TgID: tgUser.ID, Role: role, Exp: time.Now().Add(24 * time.Hour).Unix()})
	writeJSON(w, 200, map[string]any{"jwt": jwt, "role": role, "tg_id": tgUser.ID})
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
		IsActive  bool   `json:"is_active"`
		CreatedAt string `json:"created_at"`
	}
	err := h.db.QueryRow(
		`SELECT telegram_id, first_name, COALESCE(last_name,''), COALESCE(username,''),
		        COALESCE(phone,''), role, is_active, created_at
		 FROM users WHERE telegram_id=$1`, tgID,
	).Scan(&u.TgID, &u.FirstName, &u.LastName, &u.Username,
		&u.Phone, &u.Role, &u.IsActive, &u.CreatedAt)
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
	for k, q := range map[string]string{
		"users":              `SELECT COUNT(*) FROM users WHERE is_active=true`,
		"pending_clips":      `SELECT COUNT(*) FROM clip_requests WHERE status='pending'`,
		"paid_clips":         `SELECT COUNT(*) FROM clip_requests WHERE status='paid'`,
		"processing_clips":   `SELECT COUNT(*) FROM clip_requests WHERE status='processing'`,
		"active_tournaments": `SELECT COUNT(*) FROM tournaments WHERE status IN ('registration','in_progress')`,
	} {
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
	search := strings.TrimSpace(r.URL.Query().Get("q"))
	roleFilter := strings.TrimSpace(r.URL.Query().Get("role"))

	q := `SELECT telegram_id, first_name, COALESCE(last_name,''), COALESCE(username,''),
	             role, is_active, created_at FROM users WHERE 1=1`
	var args []any

	if search != "" {
		args = append(args, "%"+search+"%")
		n := strconv.Itoa(len(args))
		q += " AND (first_name ILIKE $" + n + " OR username ILIKE $" + n + ")"
	}
	validRoles := map[string]bool{"client": true, "operator": true, "admin": true, "superadmin": true}
	if roleFilter != "" && validRoles[roleFilter] {
		args = append(args, roleFilter)
		q += " AND role=$" + strconv.Itoa(len(args))
	}
	q += " ORDER BY created_at DESC LIMIT 200"

	rows, err := h.db.Query(q, args...)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type item struct {
		TgID      int64  `json:"tg_id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
		Role      string `json:"role"`
		IsActive  bool   `json:"is_active"`
		CreatedAt string `json:"created_at"`
	}
	list := []item{}
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.TgID, &i.FirstName, &i.LastName, &i.Username,
			&i.Role, &i.IsActive, &i.CreatedAt); err == nil {
			list = append(list, i)
		}
	}
	writeJSON(w, 200, list)
}

func (h *Handler) adminSetRole(w http.ResponseWriter, r *http.Request) {
	tgID, err := strconv.ParseInt(r.PathValue("tgid"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid tg_id")
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	validRoles := map[string]bool{"client": true, "operator": true, "admin": true, "superadmin": true}
	if !validRoles[req.Role] {
		writeErr(w, 400, "invalid role")
		return
	}
	// Prevent self-demotion
	caller := getClaims(r)
	if caller.TgID == tgID && req.Role != "superadmin" {
		writeErr(w, 400, "cannot change own role")
		return
	}
	result, err := h.db.Exec(`UPDATE users SET role=$1 WHERE telegram_id=$2`, req.Role, tgID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeErr(w, 404, "user not found")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) adminSetActive(w http.ResponseWriter, r *http.Request) {
	tgID, err := strconv.ParseInt(r.PathValue("tgid"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid tg_id")
		return
	}
	var req struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	// Prevent self-deactivation
	caller := getClaims(r)
	if caller.TgID == tgID && !req.Active {
		writeErr(w, 400, "cannot deactivate own account")
		return
	}
	result, err := h.db.Exec(`UPDATE users SET is_active=$1 WHERE telegram_id=$2`, req.Active, tgID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeErr(w, 404, "user not found")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) meTournamentRegister(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	trnID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	var firstName, lastName string
	_ = h.db.QueryRow(`SELECT first_name, COALESCE(last_name,'') FROM users WHERE telegram_id=$1`,
		claims.TgID).Scan(&firstName, &lastName)
	name := strings.TrimSpace(firstName + " " + lastName)
	if name == "" {
		name = "Player"
	}
	body, _ := json.Marshal(map[string]any{"user_tg_id": claims.TgID, "user_name": name})
	resp, err := http.Post(
		h.trnSvcURL+"/tournaments/"+strconv.FormatInt(trnID, 10)+"/register",
		"application/json", bytes.NewReader(body),
	)
	if err != nil {
		writeErr(w, 502, "tournament service unavailable")
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint
}

func (h *Handler) meUpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "bad request")
		return
	}
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	if req.FirstName == "" {
		writeErr(w, 400, "first_name is required")
		return
	}
	if len(req.FirstName) > 64 || len(req.LastName) > 64 {
		writeErr(w, 400, "name too long")
		return
	}
	_, err := h.db.Exec(
		`UPDATE users SET first_name=$1, last_name=NULLIF($2,'') WHERE telegram_id=$3`,
		req.FirstName, req.LastName, claims.TgID,
	)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
