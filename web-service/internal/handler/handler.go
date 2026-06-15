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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	db            *sql.DB
	secret        string
	baseURL       string
	botToken      string
	trnSvcURL     string
	clipSvcURL    string
	shopSvcURL    string
	internalToken string
}

func New(db *sql.DB, secret, baseURL, botToken, trnSvcURL, clipSvcURL, shopSvcURL, internalToken string) *Handler {
	return &Handler{db: db, secret: secret, baseURL: baseURL, botToken: botToken, trnSvcURL: trnSvcURL, clipSvcURL: clipSvcURL, shopSvcURL: shopSvcURL, internalToken: internalToken}
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

	// Tournament management (CRUD + bracket)
	mux.HandleFunc("GET /api/admin/branches", h.withRole(h.adminBranches, "operator", "admin", "superadmin"))
	mux.HandleFunc("POST /api/admin/tournaments", h.withRole(h.adminCreateTrn, "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/tournaments/{id}", h.withRole(h.adminUpdateTrn, "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/tournaments/{id}/cancel", h.withRole(h.adminCancelTrn, "admin", "superadmin"))
	mux.HandleFunc("DELETE /api/admin/tournaments/{id}", h.withRole(h.adminDeleteTrn, "admin", "superadmin"))
	mux.HandleFunc("POST /api/admin/tournaments/{id}/bracket", h.withRole(h.adminGenBracket, "admin", "superadmin"))
	mux.HandleFunc("POST /api/admin/tournaments/{id}/bracket/shuffle", h.withRole(h.adminShuffleBracket, "admin", "superadmin"))
	mux.HandleFunc("GET /api/admin/tournaments/{id}/bracket", h.withRole(h.adminGetBracket, "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/matches/{id}/result", h.withRole(h.adminMatchResult, "admin", "superadmin"))
	mux.HandleFunc("POST /api/admin/tournaments/{id}/register-manual", h.withRole(h.adminRegisterManual, "admin", "superadmin"))

	// Tournament registration management
	mux.HandleFunc("GET /api/admin/tournaments/{id}/registrations", h.withRole(h.adminTrnRegistrations, "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/tournaments/{id}/registrations/{reg_id}/approve", h.withRole(h.adminApproveTrnReg, "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/tournaments/{id}/registrations/{reg_id}/reject", h.withRole(h.adminRejectTrnReg, "admin", "superadmin"))

	// Clip management
	mux.HandleFunc("GET /api/admin/clips/{id}", h.withRole(h.adminClipDetail, "operator", "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/clips/{id}/approve", h.withRole(h.adminClipApprove, "operator", "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/clips/{id}/reject", h.withRole(h.adminClipReject, "operator", "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/clips/{id}/refund", h.withRole(h.adminClipRefund, "operator", "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/clips/{id}/record", h.withRole(h.adminClipRecord, "operator", "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/clips/{id}/done", h.withRole(h.adminClipDone, "operator", "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/clips/{id}/retry", h.withRole(h.adminClipRetry, "operator", "admin", "superadmin"))
	// Match management
	mux.HandleFunc("PUT /api/admin/matches/{id}/table", h.withRole(h.adminMatchAssignTable, "operator", "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/matches/{id}/schedule", h.withRole(h.adminMatchSchedule, "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/matches/{id}/start", h.withRole(h.adminMatchStart, "operator", "admin", "superadmin"))

	// Superadmin-only user management
	mux.HandleFunc("PUT /api/admin/users/{tgid}/role", h.withRole(h.adminSetRole, "superadmin"))
	mux.HandleFunc("PUT /api/admin/users/{tgid}/active", h.withRole(h.adminSetActive, "superadmin"))

	// Public tournament endpoints (no auth required — public data)
	mux.HandleFunc("GET /api/tournaments", h.publicTournaments)
	mux.HandleFunc("GET /api/tournaments/{id}/bracket", h.publicTrnBracket)

	// Shop — admin CRUD (JWT) → proxied to shop-service.
	// Public list (/api/products) and images (/uploads/) go straight to shop-service via nginx.
	mux.HandleFunc("GET /api/admin/products", h.withRole(h.adminListProducts, "admin", "superadmin"))
	mux.HandleFunc("POST /api/admin/products", h.withRole(h.adminCreateProduct, "admin", "superadmin"))
	mux.HandleFunc("POST /api/admin/products/image", h.withRole(h.adminUploadProductImage, "admin", "superadmin"))
	mux.HandleFunc("PUT /api/admin/products/{id}", h.withRole(h.adminUpdateProduct, "admin", "superadmin"))
	mux.HandleFunc("DELETE /api/admin/products/{id}", h.withRole(h.adminDeleteProduct, "admin", "superadmin"))

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
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(p[2])
	if err != nil || !hmac.Equal(expected, got) {
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

	// sig = HMAC-SHA256(key=secretKey, data=dataCheck)
	mac2 := hmac.New(sha256.New, secretKey)
	mac2.Write([]byte(dataCheck))
	expectedSig := mac2.Sum(nil)

	gotSig, err := hex.DecodeString(hash)
	if err != nil || !hmac.Equal(expectedSig, gotSig) {
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

	// Auto-register new users; sync username always.
	// Name synced from Telegram only if user hasn't manually customized it (name_customized=false).
	var role string
	var isActive bool
	err = h.db.QueryRow(`
		INSERT INTO users (telegram_id, username, first_name, last_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (telegram_id) DO UPDATE
		  SET username   = EXCLUDED.username,
		      first_name = CASE WHEN users.name_customized THEN users.first_name
		                        ELSE COALESCE(NULLIF(EXCLUDED.first_name,''), users.first_name) END,
		      last_name  = CASE WHEN users.name_customized THEN users.last_name
		                        ELSE COALESCE(NULLIF(EXCLUDED.last_name,''),  users.last_name)  END
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
		       t.scheduled_at, t.status, tr.status, tr.registered_at,
		       t.max_players, t.price, COALESCE(t.type,'single_elimination'),
		       (SELECT COUNT(*) FROM tournament_registrations r2
		        WHERE r2.tournament_id=t.id AND r2.status='approved') AS approved_count
		FROM tournament_registrations tr
		JOIN tournaments t ON t.id=tr.tournament_id
		LEFT JOIN branches b ON b.id=t.branch_id
		WHERE tr.user_tg_id=$1 ORDER BY tr.registered_at DESC LIMIT 50`, tgID)
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
		MaxPlayers       int    `json:"max_players"`
		Price            int64  `json:"price"`
		Type             string `json:"type"`
		ApprovedCount    int    `json:"approved_count"`
	}
	list := []item{}
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.TournamentID, &i.Name, &i.BranchName,
			&i.ScheduledAt, &i.TournamentStatus, &i.RegStatus, &i.RegisteredAt,
			&i.MaxPlayers, &i.Price, &i.Type, &i.ApprovedCount); err == nil {
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
		"pending_trn_regs":   `SELECT COUNT(*) FROM tournament_registrations WHERE status='pending'`,
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
	status := r.URL.Query().Get("status")
	q := `SELECT t.id, t.name, COALESCE(b.name,'?'), t.scheduled_at, t.status, t.max_players,
		       COALESCE(t.table_count,0),
		       COUNT(tr.id) FILTER (WHERE tr.status='approved'),
		       COUNT(tr.id) FILTER (WHERE tr.status='pending')
		FROM tournaments t
		LEFT JOIN branches b ON b.id=t.branch_id
		LEFT JOIN tournament_registrations tr ON tr.tournament_id=t.id`
	var args []any
	if status != "" {
		q += " WHERE t.status=$1"
		args = append(args, status)
	}
	q += " GROUP BY t.id, b.name ORDER BY t.created_at DESC LIMIT 50"
	rows, err := h.db.Query(q, args...)
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
		TableCount  int    `json:"table_count"`
		Registered  int    `json:"registered"`
		Pending     int    `json:"pending"`
	}
	list := []item{}
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.ID, &i.Name, &i.BranchName, &i.ScheduledAt,
			&i.Status, &i.MaxPlayers, &i.TableCount, &i.Registered, &i.Pending); err == nil {
			list = append(list, i)
		}
	}
	writeJSON(w, 200, list)
}

// ─── Tournament proxy helpers ───

func (h *Handler) trnProxy(w http.ResponseWriter, r *http.Request, method, path string, body io.Reader) {
	req, err := http.NewRequestWithContext(r.Context(), method, h.trnSvcURL+path, body)
	if err != nil {
		writeErr(w, 500, "proxy error")
		return
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if h.internalToken != "" {
		req.Header.Set("X-Internal-Token", h.internalToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeErr(w, 502, "tournament service unavailable")
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}

func (h *Handler) clipProxy(w http.ResponseWriter, r *http.Request, method, path string, body io.Reader) {
	req, err := http.NewRequestWithContext(r.Context(), method, h.clipSvcURL+path, body)
	if err != nil {
		writeErr(w, 500, "proxy error")
		return
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeErr(w, 502, "clip service unavailable")
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}

func (h *Handler) adminClipRecord(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.clipProxy(w, r, http.MethodPost, "/clips/"+id+"/record", bytes.NewReader([]byte("{}")))
}

func (h *Handler) adminClipDone(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	res, err := h.db.Exec(`UPDATE clip_requests SET status='done' WHERE id=$1 AND status IN ('paid','processing')`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeErr(w, 400, "klip topilmadi yoki holat o'zgartirish mumkin emas")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) adminClipRetry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	res, err := h.db.Exec(`UPDATE clip_requests SET status='paid', notes='' WHERE id=$1 AND status='failed'`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeErr(w, 400, "klip topilmadi yoki rad etilmagan emas")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) adminMatchAssignTable(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	matchID, _ := strconv.ParseInt(id, 10, 64)

	body, _ := io.ReadAll(r.Body)
	var req struct {
		TableNum int `json:"table_num"`
	}
	_ = json.Unmarshal(body, &req)

	tReq, err := http.NewRequestWithContext(r.Context(), http.MethodPut,
		h.trnSvcURL+"/matches/"+id+"/table", bytes.NewReader(body))
	if err != nil {
		writeErr(w, 500, "internal error")
		return
	}
	tReq.Header.Set("Content-Type", "application/json")
	tReq.Header.Set("X-Internal-Token", h.internalToken)
	resp, err := http.DefaultClient.Do(tReq)
	if err != nil {
		writeErr(w, 502, "tournament service unavailable")
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
		return
	}

	go h.sendMatchTableNotifications(matchID, req.TableNum)
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) sendMatchTableNotifications(matchID int64, tableNum int) {
	var p1TgID, p2TgID *int64
	var p1Name, p2Name, trnName string
	var matchScheduledAt *time.Time
	err := h.db.QueryRow(`
		SELECT m.player1_tg_id, m.player2_tg_id,
		       COALESCE(r1.user_name, u1.first_name, ''), COALESCE(r2.user_name, u2.first_name, ''),
		       t.name, m.match_scheduled_at
		FROM tournament_matches m
		JOIN tournaments t ON t.id=m.tournament_id
		LEFT JOIN tournament_registrations r1 ON r1.tournament_id=m.tournament_id AND r1.user_tg_id=m.player1_tg_id
		LEFT JOIN tournament_registrations r2 ON r2.tournament_id=m.tournament_id AND r2.user_tg_id=m.player2_tg_id
		LEFT JOIN users u1 ON u1.telegram_id=m.player1_tg_id
		LEFT JOIN users u2 ON u2.telegram_id=m.player2_tg_id
		WHERE m.id=$1`, matchID).Scan(&p1TgID, &p2TgID, &p1Name, &p2Name, &trnName, &matchScheduledAt)
	if err != nil {
		return
	}

	timeStr := ""
	if matchScheduledAt != nil {
		tz := time.FixedZone("UZT", 5*3600)
		timeStr = "\n🕐 Vaqt: <b>" + matchScheduledAt.In(tz).Format("02.01.2006 15:04") + "</b>"
	}

	notify := func(tgID int64, opponentName string) {
		if tgID <= 0 {
			return
		}
		msg := fmt.Sprintf(
			"⚡ <b>Navbatdagi o'yiningiz!</b>\n\n🏆 Turnir: <b>%s</b>\n🆚 Raqibingiz: <b>%s</b>\n🎱 Stol: <b>%d-stol</b>%s\n\nOmad! 🍀",
			trnName, opponentName, tableNum, timeStr)
		h.sendTelegramMessage(tgID, msg, nil)
	}

	if p1TgID != nil {
		notify(*p1TgID, p2Name)
	}
	if p2TgID != nil {
		notify(*p2TgID, p1Name)
	}
}

func (h *Handler) adminMatchStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.trnProxy(w, r, http.MethodPut, "/matches/"+id+"/start", r.Body)
}

func (h *Handler) adminMatchSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.trnProxy(w, r, http.MethodPut, "/matches/"+id+"/schedule", r.Body)
}

func (h *Handler) adminBranches(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, name FROM branches ORDER BY id`)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type item struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	list := []item{}
	for rows.Next() {
		var i item
		if rows.Scan(&i.ID, &i.Name) == nil {
			list = append(list, i)
		}
	}
	writeJSON(w, 200, list)
}

func (h *Handler) adminCreateTrn(w http.ResponseWriter, r *http.Request) {
	claims := getClaims(r)
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, 400, "invalid request")
		return
	}
	req["admin_tg_id"] = claims.TgID
	if req["type"] == nil || req["type"] == "" {
		req["type"] = "single_elimination"
	}
	body, _ := json.Marshal(req)
	h.trnProxy(w, r, http.MethodPost, "/tournaments", bytes.NewReader(body))
}

func (h *Handler) adminUpdateTrn(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.trnProxy(w, r, http.MethodPut, "/tournaments/"+id, r.Body)
}

func (h *Handler) adminCancelTrn(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.trnProxy(w, r, http.MethodPut, "/tournaments/"+id+"/cancel", bytes.NewReader([]byte("{}")))
}

func (h *Handler) adminDeleteTrn(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.trnProxy(w, r, http.MethodDelete, "/tournaments/"+id, nil)
}

func (h *Handler) adminGenBracket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.genBracketAndNotify(w, r, id, "/tournaments/"+id+"/bracket")
}

func (h *Handler) adminShuffleBracket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.genBracketAndNotify(w, r, id, "/tournaments/"+id+"/bracket/shuffle")
}

func (h *Handler) genBracketAndNotify(w http.ResponseWriter, r *http.Request, trnID, path string) {
	tReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		h.trnSvcURL+path, bytes.NewReader([]byte("{}")))
	if err != nil {
		writeErr(w, 500, "internal error")
		return
	}
	tReq.Header.Set("Content-Type", "application/json")
	tReq.Header.Set("X-Internal-Token", h.internalToken)
	resp, err := http.DefaultClient.Do(tReq)
	if err != nil {
		writeErr(w, 502, "tournament service unavailable")
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
	if resp.StatusCode != 200 {
		return
	}
	go func() {
		type mEntry struct {
			ID          int64  `json:"id"`
			Round       int    `json:"round"`
			MatchNum    int    `json:"match_num"`
			Status      string `json:"status"`
			TableNum    int    `json:"table_num"`
			Player1TgID *int64 `json:"player1_tg_id"`
			Player2TgID *int64 `json:"player2_tg_id"`
			Player1Name string `json:"player1_name"`
			Player2Name string `json:"player2_name"`
		}
		var matches []mEntry
		if json.Unmarshal(respBody, &matches) != nil {
			return
		}

		// Auto-assigned table notifications (existing logic)
		for _, m := range matches {
			if m.Status == "ready" && m.TableNum > 0 {
				h.sendMatchTableNotifications(m.ID, m.TableNum)
			}
		}

		// Bracket-created notification to all approved participants
		trnIDInt, _ := strconv.ParseInt(trnID, 10, 64)
		var trnName string
		if h.db.QueryRow(`SELECT name FROM tournaments WHERE id=$1`, trnIDInt).Scan(&trnName) != nil {
			return
		}

		// Find the minimum WB round (round < 1000) for first-match info
		minRound := 0
		for _, m := range matches {
			if m.Round > 0 && m.Round < 1000 {
				if minRound == 0 || m.Round < minRound {
					minRound = m.Round
				}
			}
		}

		type firstMatchInfo struct{ opponent string; tableNum int }
		playerFirst := map[int64]firstMatchInfo{}
		for _, m := range matches {
			if m.Round != minRound || minRound == 0 {
				continue
			}
			if m.Player1TgID != nil && *m.Player1TgID > 0 {
				if _, ok := playerFirst[*m.Player1TgID]; !ok {
					playerFirst[*m.Player1TgID] = firstMatchInfo{m.Player2Name, m.TableNum}
				}
			}
			if m.Player2TgID != nil && *m.Player2TgID > 0 {
				if _, ok := playerFirst[*m.Player2TgID]; !ok {
					playerFirst[*m.Player2TgID] = firstMatchInfo{m.Player1Name, m.TableNum}
				}
			}
		}

		rows, err := h.db.Query(`SELECT user_tg_id FROM tournament_registrations WHERE tournament_id=$1 AND status='approved'`, trnIDInt)
		if err != nil {
			return
		}
		defer rows.Close()
		for rows.Next() {
			var tgID int64
			if rows.Scan(&tgID) != nil || tgID <= 0 {
				continue
			}
			var msg string
			if info, ok := playerFirst[tgID]; ok {
				opponent := info.opponent
				if opponent == "" {
					opponent = "Noma'lum"
				}
				tableStr := ""
				if info.tableNum > 0 {
					tableStr = fmt.Sprintf("\n🎱 Stol: <b>%d-stol</b>", info.tableNum)
				}
				msg = fmt.Sprintf("⚡ <b>%s</b> turniri sеtkasi yaratildi!\n\nSizning birinchi o'yiningiz:\n🆚 Raqib: <b>%s</b>%s\n\nOmad! 🍀", trnName, opponent, tableStr)
			} else {
				msg = fmt.Sprintf("⚡ <b>%s</b> turniri sеtkasi yaratildi!\n\nJadval va o'yin ma'lumotlari web saytda mavjud. Omad! 🍀", trnName)
			}
			go h.sendTelegramMessage(tgID, msg, nil)
		}
	}()
}

func (h *Handler) adminGetBracket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.trnProxy(w, r, http.MethodGet, "/tournaments/"+id+"/bracket", nil)
}

func (h *Handler) publicTrnBracket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.trnProxy(w, r, http.MethodGet, "/tournaments/"+id+"/bracket", nil)
}

func (h *Handler) publicTournaments(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	q := `SELECT t.id, t.name, COALESCE(b.name,'?'), t.scheduled_at, t.status, t.max_players,
		       COALESCE(t.type,'single_elimination'), COALESCE(t.table_count,0),
		       COUNT(tr.id) FILTER (WHERE tr.status='approved'),
		       COUNT(tr.id) FILTER (WHERE tr.status='pending')
		FROM tournaments t
		LEFT JOIN branches b ON b.id=t.branch_id
		LEFT JOIN tournament_registrations tr ON tr.tournament_id=t.id`
	var args []any
	if status != "" {
		q += " WHERE t.status=$1"
		args = append(args, status)
	} else {
		q += " WHERE t.status IN ('registration','in_progress','finished')"
	}
	q += " GROUP BY t.id, b.name ORDER BY t.scheduled_at DESC LIMIT 50"
	rows, err := h.db.Query(q, args...)
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
		Type        string `json:"type"`
		TableCount  int    `json:"table_count"`
		Registered  int    `json:"registered"`
		Pending     int    `json:"pending"`
	}
	list := []item{}
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.ID, &i.Name, &i.BranchName, &i.ScheduledAt,
			&i.Status, &i.MaxPlayers, &i.Type, &i.TableCount, &i.Registered, &i.Pending); err == nil {
			list = append(list, i)
		}
	}
	writeJSON(w, 200, list)
}

// ─── Shop / Products → shop-service proxy (admin, JWT-gated) ───

func (h *Handler) shopProxy(w http.ResponseWriter, r *http.Request, method, path string, body io.Reader, contentType string) {
	req, err := http.NewRequestWithContext(r.Context(), method, h.shopSvcURL+path, body)
	if err != nil {
		writeErr(w, 500, "proxy error")
		return
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if h.internalToken != "" {
		req.Header.Set("X-Internal-Token", h.internalToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeErr(w, 502, "shop service unavailable")
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
}

func (h *Handler) adminListProducts(w http.ResponseWriter, r *http.Request) {
	h.shopProxy(w, r, http.MethodGet, "/products/all", nil, "")
}

func (h *Handler) adminCreateProduct(w http.ResponseWriter, r *http.Request) {
	h.shopProxy(w, r, http.MethodPost, "/products", r.Body, "application/json")
}

func (h *Handler) adminUpdateProduct(w http.ResponseWriter, r *http.Request) {
	h.shopProxy(w, r, http.MethodPut, "/products/"+r.PathValue("id"), r.Body, "application/json")
}

func (h *Handler) adminDeleteProduct(w http.ResponseWriter, r *http.Request) {
	h.shopProxy(w, r, http.MethodDelete, "/products/"+r.PathValue("id"), nil, "")
}

// adminUploadProductImage streams the multipart upload through to shop-service.
func (h *Handler) adminUploadProductImage(w http.ResponseWriter, r *http.Request) {
	h.shopProxy(w, r, http.MethodPost, "/products/image", r.Body, r.Header.Get("Content-Type"))
}

func (h *Handler) adminMatchResult(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	body, _ := io.ReadAll(r.Body)
	tReq, err := http.NewRequestWithContext(r.Context(), http.MethodPut,
		h.trnSvcURL+"/matches/"+id+"/result", bytes.NewReader(body))
	if err != nil {
		writeErr(w, 500, "internal error")
		return
	}
	tReq.Header.Set("Content-Type", "application/json")
	tReq.Header.Set("X-Internal-Token", h.internalToken)
	resp, err := http.DefaultClient.Do(tReq)
	if err != nil {
		writeErr(w, 502, "tournament service unavailable")
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)
	if resp.StatusCode != 200 {
		return
	}
	matchID, _ := strconv.ParseInt(id, 10, 64)
	go func() {
		var res struct {
			NextMatchID      int64 `json:"next_match_id"`
			LoserNextMatchID int64 `json:"loser_next_match_id"`
			Finished         bool  `json:"finished"`
			AssignedMatchID  int64 `json:"assigned_match_id"`
			AssignedTableNum int   `json:"assigned_table_num"`
		}
		if json.Unmarshal(respBody, &res) != nil {
			return
		}
		// 1) G'olib va mag'lubga natija xabari (+ keyingi raqib)
		h.sendResultNotifications(matchID, res.NextMatchID, res.Finished)
		// 2) Bo'shagan stol keyingi o'yinga berilsa — o'sha o'yinchilarga xabar
		if res.AssignedMatchID > 0 {
			h.sendMatchTableNotifications(res.AssignedMatchID, res.AssignedTableNum)
		}
	}()
}

// sendResultNotifications — o'yin tugaganda g'olib va mag'lubga xabar yuboradi.
func (h *Handler) sendResultNotifications(matchID, nextMatchID int64, finished bool) {
	var p1TgID, p2TgID, winnerTgID, loserNextID *int64
	var p1Name, p2Name, trnName, trnType string
	var p1Score, p2Score int
	err := h.db.QueryRow(`
		SELECT m.player1_tg_id, m.player2_tg_id, m.winner_tg_id, m.loser_next_match_id,
		       COALESCE(r1.user_name, u1.first_name, ''), COALESCE(r2.user_name, u2.first_name, ''),
		       t.name, COALESCE(t.type,'single_elimination'),
		       COALESCE(m.player1_score,0), COALESCE(m.player2_score,0)
		FROM tournament_matches m
		JOIN tournaments t ON t.id=m.tournament_id
		LEFT JOIN tournament_registrations r1 ON r1.tournament_id=m.tournament_id AND r1.user_tg_id=m.player1_tg_id
		LEFT JOIN tournament_registrations r2 ON r2.tournament_id=m.tournament_id AND r2.user_tg_id=m.player2_tg_id
		LEFT JOIN users u1 ON u1.telegram_id=m.player1_tg_id
		LEFT JOIN users u2 ON u2.telegram_id=m.player2_tg_id
		WHERE m.id=$1`, matchID).Scan(
		&p1TgID, &p2TgID, &winnerTgID, &loserNextID,
		&p1Name, &p2Name, &trnName, &trnType, &p1Score, &p2Score)
	if err != nil || winnerTgID == nil {
		return
	}

	// G'olib/mag'lubni aniqlash
	var winTg, loseTg *int64
	var winName, loseName string
	var winScore, loseScore int
	if p1TgID != nil && *p1TgID == *winnerTgID {
		winTg, loseTg = p1TgID, p2TgID
		winName, loseName = p1Name, p2Name
		winScore, loseScore = p1Score, p2Score
	} else {
		winTg, loseTg = p2TgID, p1TgID
		winName, loseName = p2Name, p1Name
		winScore, loseScore = p2Score, p1Score
	}

	scoreStr := ""
	if p1Score > 0 || p2Score > 0 {
		scoreStr = fmt.Sprintf(" <b>%d:%d</b>", winScore, loseScore)
	}
	if loseName == "" {
		loseName = "raqib"
	}
	if winName == "" {
		winName = "raqib"
	}

	// G'olibga xabar
	if winTg != nil && *winTg > 0 {
		var tail string
		if finished {
			tail = "\n\n👑 <b>Siz TURNIR CHEMPIONISIZ!</b> 🎉🏆"
		} else if nextMatchID > 0 {
			if opp := h.nextOpponentName(nextMatchID, *winTg); opp != "" {
				tail = fmt.Sprintf("\n\n⏭ Keyingi o'yin: <b>%s</b> bilan. Tayyorlaning!", opp)
			} else {
				tail = "\n\n⏭ Keyingi bosqichga o'tdingiz! Raqibingiz aniqlanmoqda…"
			}
		}
		msg := fmt.Sprintf("🏆 <b>Tabriklaymiz!</b>\n\n<b>%s</b> turnirida <b>%s</b> ustidan g'alaba qozondingiz%s.%s",
			trnName, loseName, scoreStr, tail)
		h.sendTelegramMessage(*winTg, msg, nil)
	}

	// Mag'lubga xabar
	if loseTg != nil && *loseTg > 0 {
		var msg string
		if trnType == "double_elimination" && loserNextID != nil && *loserNextID > 0 {
			msg = fmt.Sprintf("😔 <b>%s</b> turnirida <b>%s</b> ga yutqazdingiz%s.\n\n💪 Hali imkoniyat bor — <b>yutqazuvchilar setkasida</b> davom etasiz!",
				trnName, winName, scoreStr)
		} else {
			msg = fmt.Sprintf("😔 <b>%s</b> turnirida <b>%s</b> ga yutqazdingiz%s.\n\nO'yiningiz uchun rahmat! Keyingi turnirlarda ko'rishamiz. 🎱",
				trnName, winName, scoreStr)
		}
		h.sendTelegramMessage(*loseTg, msg, nil)
	}

	// Keyingi o'yin tayyor bo'lsa — kutayotgan raqibga ham xabar
	if !finished && nextMatchID > 0 {
		h.notifyWaitingOpponent(nextMatchID, *winnerTgID, winName, trnName)
	}
}

// nextOpponentName — keyingi o'yinda myTgID ning raqibi ismini qaytaradi (agar ma'lum bo'lsa).
func (h *Handler) nextOpponentName(nextMatchID, myTgID int64) string {
	var p1TgID, p2TgID *int64
	var p1Name, p2Name string
	err := h.db.QueryRow(`
		SELECT m.player1_tg_id, m.player2_tg_id,
		       COALESCE(r1.user_name, u1.first_name, ''), COALESCE(r2.user_name, u2.first_name, '')
		FROM tournament_matches m
		LEFT JOIN tournament_registrations r1 ON r1.tournament_id=m.tournament_id AND r1.user_tg_id=m.player1_tg_id
		LEFT JOIN tournament_registrations r2 ON r2.tournament_id=m.tournament_id AND r2.user_tg_id=m.player2_tg_id
		LEFT JOIN users u1 ON u1.telegram_id=m.player1_tg_id
		LEFT JOIN users u2 ON u2.telegram_id=m.player2_tg_id
		WHERE m.id=$1`, nextMatchID).Scan(&p1TgID, &p2TgID, &p1Name, &p2Name)
	if err != nil {
		return ""
	}
	if p1TgID != nil && *p1TgID == myTgID {
		if p2TgID != nil {
			return p2Name
		}
	} else if p2TgID != nil && *p2TgID == myTgID {
		if p1TgID != nil {
			return p1Name
		}
	}
	return ""
}

// notifyWaitingOpponent — keyingi o'yin ikkala o'yinchi bilan to'lganida, kutayotgan
// (yangi kelgan g'olibdan boshqa) o'yinchiga raqibi aniqlanganini bildiradi.
func (h *Handler) notifyWaitingOpponent(nextMatchID, winnerTgID int64, winnerName, trnName string) {
	var p1TgID, p2TgID *int64
	var status string
	err := h.db.QueryRow(
		`SELECT player1_tg_id, player2_tg_id, status FROM tournament_matches WHERE id=$1`,
		nextMatchID).Scan(&p1TgID, &p2TgID, &status)
	if err != nil || status != "ready" {
		return // ikkala o'yinchi hali to'lmagan
	}
	var waitTg *int64
	if p1TgID != nil && *p1TgID != winnerTgID {
		waitTg = p1TgID
	} else if p2TgID != nil && *p2TgID != winnerTgID {
		waitTg = p2TgID
	}
	if waitTg != nil && *waitTg > 0 {
		msg := fmt.Sprintf("🎯 <b>%s</b> turnirida raqibingiz aniqlandi: <b>%s</b>.\n\nTayyorlaning! O'yin vaqti va stoli haqida xabar beriladi.",
			trnName, winnerName)
		h.sendTelegramMessage(*waitTg, msg, nil)
	}
}

func (h *Handler) adminRegisterManual(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	h.trnProxy(w, r, http.MethodPost, "/tournaments/"+id+"/register-manual", r.Body)
}

// ─── Tournament registration management ───

func (h *Handler) adminTrnRegistrations(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	rows, err := h.db.Query(`
		SELECT id, user_tg_id, user_name, status, registered_at,
		       COALESCE(decided_at::text, '')
		FROM tournament_registrations
		WHERE tournament_id=$1
		ORDER BY
		    CASE status WHEN 'pending' THEN 0 WHEN 'approved' THEN 1 ELSE 2 END,
		    registered_at DESC`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	type item struct {
		ID           int64  `json:"id"`
		UserTgID     int64  `json:"user_tg_id"`
		UserName     string `json:"user_name"`
		Status       string `json:"status"`
		RegisteredAt string `json:"registered_at"`
		DecidedAt    string `json:"decided_at"`
	}
	list := []item{}
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.ID, &i.UserTgID, &i.UserName, &i.Status,
			&i.RegisteredAt, &i.DecidedAt); err == nil {
			list = append(list, i)
		}
	}
	writeJSON(w, 200, list)
}

func (h *Handler) adminApproveTrnReg(w http.ResponseWriter, r *http.Request) {
	tID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	rID, err := strconv.ParseInt(r.PathValue("reg_id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid reg_id")
		return
	}
	var tStatus string
	_ = h.db.QueryRow(`SELECT status FROM tournaments WHERE id=$1`, tID).Scan(&tStatus)
	if tStatus != "registration" {
		writeErr(w, 400, "turnir ro'yxat qabul qilmayapti")
		return
	}
	var clientTgID int64
	var trnName, scheduledAt string
	_ = h.db.QueryRow(`
		SELECT tr.user_tg_id, t.name, COALESCE(t.scheduled_at::text,'')
		FROM tournament_registrations tr
		JOIN tournaments t ON t.id=tr.tournament_id
		WHERE tr.id=$1 AND tr.tournament_id=$2`, rID, tID).Scan(&clientTgID, &trnName, &scheduledAt)

	_, err = h.db.Exec(`
		UPDATE tournament_registrations SET status='approved', decided_at=NOW()
		WHERE id=$1 AND tournament_id=$2 AND status='pending'`, rID, tID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if clientTgID > 0 {
		msg := fmt.Sprintf("✅ <b>%s</b> turnirida ro'yxatingiz tasdiqlandi!\n\nSiz ishtirokchisiz. Turnir boshlanishi haqida xabardor qilinasiz.", trnName)
		go h.sendTelegramMessage(clientTgID, msg, nil)
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) adminRejectTrnReg(w http.ResponseWriter, r *http.Request) {
	tID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	rID, err := strconv.ParseInt(r.PathValue("reg_id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid reg_id")
		return
	}
	var clientTgID int64
	var trnName string
	_ = h.db.QueryRow(`
		SELECT tr.user_tg_id, t.name
		FROM tournament_registrations tr
		JOIN tournaments t ON t.id=tr.tournament_id
		WHERE tr.id=$1 AND tr.tournament_id=$2`, rID, tID).Scan(&clientTgID, &trnName)

	_, err = h.db.Exec(`
		UPDATE tournament_registrations SET status='rejected', decided_at=NOW()
		WHERE id=$1 AND tournament_id=$2 AND status='pending'`, rID, tID)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	if clientTgID > 0 {
		msg := fmt.Sprintf("❌ <b>%s</b> turnirida ro'yxatingiz rad etildi.\n\nQo'shimcha ma'lumot uchun admin bilan bog'laning.", trnName)
		go h.sendTelegramMessage(clientTgID, msg, nil)
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) adminClipDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	type item struct {
		ID         int64  `json:"id"`
		ClientTgID int64  `json:"client_tg_id"`
		ClientName string `json:"client_name"`
		BranchName string `json:"branch_name"`
		TableNum   int    `json:"table_num"`
		StartTime  string `json:"start_time"`
		EndTime    string `json:"end_time"`
		Status     string `json:"status"`
		Notes      string `json:"notes"`
		ClipPath   string `json:"clip_path"`
		CreatedAt  string `json:"created_at"`
	}
	var i item
	err = h.db.QueryRow(`
		SELECT cr.id, cr.client_tg_id, cr.client_name, COALESCE(b.name,'?'), COALESCE(t.table_num,0),
		       cr.start_time, cr.end_time, cr.status, COALESCE(cr.notes,''), COALESCE(cr.clip_path,''), cr.created_at
		FROM clip_requests cr
		LEFT JOIN branches b ON b.id=cr.branch_id
		LEFT JOIN tables t ON t.id=cr.table_id
		WHERE cr.id=$1`, id).Scan(
		&i.ID, &i.ClientTgID, &i.ClientName, &i.BranchName, &i.TableNum,
		&i.StartTime, &i.EndTime, &i.Status, &i.Notes, &i.ClipPath, &i.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeErr(w, 404, "not found")
		return
	}
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, i)
}

func (h *Handler) adminClipApprove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	res, err := h.db.Exec(`UPDATE clip_requests SET status='paid' WHERE id=$1 AND status='pending'`, id)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeErr(w, 400, "klip topilmadi yoki pending emas")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) adminClipReject(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	var req struct {
		Notes string `json:"notes"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	res, err := h.db.Exec(`
		UPDATE clip_requests SET status='failed', notes=$2
		WHERE id=$1 AND status IN ('pending','paid','processing')`, id, req.Notes)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeErr(w, 400, "klip topilmadi yoki holat o'zgartirish mumkin emas")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (h *Handler) adminClipRefund(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, "invalid id")
		return
	}
	var req struct {
		Notes string `json:"notes"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	res, err := h.db.Exec(`
		UPDATE clip_requests SET status='refunded', notes=$2
		WHERE id=$1 AND status IN ('paid','processing')`, id, req.Notes)
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		writeErr(w, 400, "klip topilmadi yoki qaytarish mumkin emas")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
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

	var firstName, lastName, username, phone string
	_ = h.db.QueryRow(
		`SELECT first_name, COALESCE(last_name,''), COALESCE(username,''), COALESCE(phone,'') FROM users WHERE telegram_id=$1`,
		claims.TgID).Scan(&firstName, &lastName, &username, &phone)
	name := strings.TrimSpace(firstName + " " + lastName)
	if name == "" {
		name = "Player"
	}

	body, _ := json.Marshal(map[string]any{"user_tg_id": claims.TgID, "user_name": name})
	regReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost,
		h.trnSvcURL+"/tournaments/"+strconv.FormatInt(trnID, 10)+"/register",
		bytes.NewReader(body))
	if err != nil {
		writeErr(w, 500, "proxy error")
		return
	}
	regReq.Header.Set("Content-Type", "application/json")
	if h.internalToken != "" {
		regReq.Header.Set("X-Internal-Token", h.internalToken)
	}
	resp, err := http.DefaultClient.Do(regReq)
	if err != nil {
		writeErr(w, 502, "tournament service unavailable")
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody) //nolint

	if resp.StatusCode == 200 && h.botToken != "" {
		var regResp struct {
			ID int64 `json:"id"`
		}
		_ = json.Unmarshal(respBody, &regResp)

		var trnName string
		_ = h.db.QueryRow(`SELECT name FROM tournaments WHERE id=$1`, trnID).Scan(&trnName)
		if trnName == "" {
			trnName = strconv.FormatInt(trnID, 10)
		}

		fullName := strings.TrimSpace(firstName + " " + lastName)
		if fullName == "" {
			fullName = "Player"
		}
		usernameInfo := ""
		if username != "" {
			usernameInfo = "  @" + username
		}
		phoneInfo := ""
		if phone != "" {
			phoneInfo = "\n📱 " + phone
		}
		text := "🔔 <b>Yangi turnir so'rovi</b>\n\n" +
			"🏆 <b>" + trnName + "</b>\n" +
			"👤 <b>" + fullName + "</b>" + usernameInfo +
			phoneInfo + "\n" +
			"🆔 <code>" + strconv.FormatInt(claims.TgID, 10) + "</code>\n\n" +
			"Tasdiqlash yoki rad etishingiz mumkin:"

		approveData := strconv.FormatInt(trnID, 10) + ":" + strconv.FormatInt(regResp.ID, 10)
		kb, _ := json.Marshal(map[string]any{
			"inline_keyboard": [][]map[string]any{
				{
					{"text": "✅ Tasdiqlash", "callback_data": "admin_trn_approve:" + approveData},
					{"text": "❌ Rad etish", "callback_data": "admin_trn_reject:" + approveData},
				},
				{
					{"text": "✉️ Xabar yozish", "url": "tg://user?id=" + strconv.FormatInt(claims.TgID, 10)},
					{"text": "👥 Barcha so'rovlar", "callback_data": "admin_trn_regs:" + strconv.FormatInt(trnID, 10)},
				},
			},
		})

		rows, _ := h.db.Query(`SELECT telegram_id FROM users WHERE role IN ('admin','superadmin') AND is_active=true`)
		if rows != nil {
			defer rows.Close()
			for rows.Next() {
				var adminTgID int64
				if rows.Scan(&adminTgID) != nil {
					continue
				}
				go h.sendTelegramMessage(adminTgID, text, kb)
			}
		}
	}
}

func (h *Handler) sendTelegramMessage(chatID int64, text string, replyMarkup []byte) {
	payload, _ := json.Marshal(map[string]any{
		"chat_id":      chatID,
		"text":         text,
		"parse_mode":   "HTML",
		"reply_markup": json.RawMessage(replyMarkup),
	})
	apiURL := "https://api.telegram.org/bot" + h.botToken + "/sendMessage"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
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
		`UPDATE users SET first_name=$1, last_name=NULLIF($2,''), name_customized=true WHERE telegram_id=$3`,
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
