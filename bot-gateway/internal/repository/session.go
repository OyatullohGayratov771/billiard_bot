package repository

import (
	"database/sql"
	"errors"
	"time"

	"bot-gateway/internal/models"
)

type SessionRepo struct {
	db *sql.DB
}

func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(tableID, operatorID int64, clientName string) (*models.Session, error) {
	s := &models.Session{}
	err := r.db.QueryRow(`
		INSERT INTO sessions (table_id, operator_id, client_name)
		VALUES ($1, $2, $3)
		RETURNING id, table_id, operator_id, client_name,
		          started_at, ended_at, total_min, total_price, created_at
	`, tableID, operatorID, clientName).Scan(
		&s.ID, &s.TableID, &s.OperatorID, &s.ClientName,
		&s.StartedAt, &s.EndedAt, &s.TotalMin, &s.TotalPrice, &s.CreatedAt,
	)
	return s, err
}

func (r *SessionRepo) End(sessionID int64) (*models.Session, error) {
	now := time.Now()
	s := &models.Session{}
	err := r.db.QueryRow(`
		UPDATE sessions
		SET ended_at    = $1,
		    total_min   = EXTRACT(EPOCH FROM ($1 - started_at))::INT / 60,
		    total_price = (
		        EXTRACT(EPOCH FROM ($1 - started_at))::BIGINT / 3600
		        * (SELECT price_per_hour FROM tables WHERE id = table_id)
		    )
		WHERE id = $2 AND ended_at IS NULL
		RETURNING id, table_id, operator_id, client_name,
		          started_at, ended_at, total_min, total_price, created_at
	`, now, sessionID).Scan(
		&s.ID, &s.TableID, &s.OperatorID, &s.ClientName,
		&s.StartedAt, &s.EndedAt, &s.TotalMin, &s.TotalPrice, &s.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// ActiveByTable — stol uchun faol sessiyani topadi
func (r *SessionRepo) ActiveByTable(tableID int64) (*models.Session, error) {
	s := &models.Session{}
	err := r.db.QueryRow(`
		SELECT s.id, s.table_id, s.operator_id, s.client_name,
		       s.started_at, s.ended_at, s.total_min, s.total_price, s.created_at,
		       t.table_num, b.name
		FROM sessions s
		JOIN tables t ON t.id = s.table_id
		JOIN branches b ON b.id = t.branch_id
		WHERE s.table_id = $1 AND s.ended_at IS NULL
		ORDER BY s.started_at DESC LIMIT 1
	`, tableID).Scan(
		&s.ID, &s.TableID, &s.OperatorID, &s.ClientName,
		&s.StartedAt, &s.EndedAt, &s.TotalMin, &s.TotalPrice, &s.CreatedAt,
		&s.TableNum, &s.BranchName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// GetByID — sessiyani ID bo'yicha topadi
func (r *SessionRepo) GetByID(id int64) (*models.Session, error) {
	s := &models.Session{}
	err := r.db.QueryRow(`
		SELECT s.id, s.table_id, s.operator_id, s.client_name,
		       s.started_at, s.ended_at, s.total_min, s.total_price, s.created_at,
		       t.table_num, b.name
		FROM sessions s
		JOIN tables t ON t.id = s.table_id
		JOIN branches b ON b.id = t.branch_id
		WHERE s.id = $1
	`, id).Scan(
		&s.ID, &s.TableID, &s.OperatorID, &s.ClientName,
		&s.StartedAt, &s.EndedAt, &s.TotalMin, &s.TotalPrice, &s.CreatedAt,
		&s.TableNum, &s.BranchName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return s, err
}

// DailyReport — kun hisoboti
func (r *SessionRepo) DailyReport(branchID int64) (totalSessions int, totalPriceSom int64, err error) {
	query := `
		SELECT COUNT(*), COALESCE(SUM(s.total_price),0)
		FROM sessions s
		JOIN tables t ON t.id = s.table_id
		WHERE t.branch_id = $1
		  AND s.ended_at IS NOT NULL
		  AND DATE(s.started_at) = CURRENT_DATE
	`
	err = r.db.QueryRow(query, branchID).Scan(&totalSessions, &totalPriceSom)
	totalPriceSom = totalPriceSom / 100
	return
}

// MonthlyReport — oylik hisobot
func (r *SessionRepo) MonthlyReport(branchID int64) (totalSessions int, totalPriceSom int64, err error) {
	query := `
		SELECT COUNT(*), COALESCE(SUM(s.total_price),0)
		FROM sessions s
		JOIN tables t ON t.id = s.table_id
		WHERE t.branch_id = $1
		  AND s.ended_at IS NOT NULL
		  AND DATE_TRUNC('month', s.started_at) = DATE_TRUNC('month', CURRENT_DATE)
	`
	err = r.db.QueryRow(query, branchID).Scan(&totalSessions, &totalPriceSom)
	totalPriceSom = totalPriceSom / 100
	return
}
