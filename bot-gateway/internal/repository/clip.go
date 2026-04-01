package repository

import (
	"database/sql"
	"errors"
	"time"

	"bot-gateway/internal/models"
)

type ClipRepo struct {
	db *sql.DB
}

func NewClipRepo(db *sql.DB) *ClipRepo {
	return &ClipRepo{db: db}
}

func (r *ClipRepo) Create(in models.ClipRequestInput) (*models.ClipRequest, error) {
	cr := &models.ClipRequest{}
	err := r.db.QueryRow(`
		INSERT INTO clip_requests
		    (client_tg_id, client_name, branch_id, table_id, requested_time, duration_sec)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, client_tg_id, client_name, branch_id, table_id,
		          requested_time, duration_sec, status, COALESCE(clip_path,''),
		          COALESCE(notes,''), created_at
	`, in.ClientTgID, in.ClientName, in.BranchID, in.TableID,
		in.RequestedTime, in.DurationSec,
	).Scan(
		&cr.ID, &cr.ClientTgID, &cr.ClientName, &cr.BranchID, &cr.TableID,
		&cr.RequestedTime, &cr.DurationSec, &cr.Status, &cr.ClipPath,
		&cr.Notes, &cr.CreatedAt,
	)
	return cr, err
}

func (r *ClipRepo) GetByID(id int64) (*models.ClipRequest, error) {
	cr := &models.ClipRequest{}
	err := r.db.QueryRow(`
		SELECT cr.id, cr.client_tg_id, cr.client_name, cr.branch_id, cr.table_id,
		       cr.requested_time, cr.duration_sec, cr.status,
		       COALESCE(cr.clip_path,''), COALESCE(cr.notes,''), cr.created_at,
		       b.name, t.table_num
		FROM clip_requests cr
		JOIN branches b ON b.id = cr.branch_id
		JOIN tables t ON t.id = cr.table_id
		WHERE cr.id = $1
	`, id).Scan(
		&cr.ID, &cr.ClientTgID, &cr.ClientName, &cr.BranchID, &cr.TableID,
		&cr.RequestedTime, &cr.DurationSec, &cr.Status, &cr.ClipPath,
		&cr.Notes, &cr.CreatedAt, &cr.BranchName, &cr.TableNum,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return cr, err
}

func (r *ClipRepo) SetStatus(id int64, status string) error {
	_, err := r.db.Exec(
		`UPDATE clip_requests SET status = $1 WHERE id = $2`,
		status, id,
	)
	return err
}

func (r *ClipRepo) ListByClient(tgID int64) ([]*models.ClipRequest, error) {
	rows, err := r.db.Query(`
		SELECT cr.id, cr.client_tg_id, cr.client_name, cr.branch_id, cr.table_id,
		       cr.requested_time, cr.duration_sec, cr.status,
		       COALESCE(cr.clip_path,''), COALESCE(cr.notes,''), cr.created_at,
		       b.name, t.table_num
		FROM clip_requests cr
		JOIN branches b ON b.id = cr.branch_id
		JOIN tables t ON t.id = cr.table_id
		WHERE cr.client_tg_id = $1
		ORDER BY cr.created_at DESC
		LIMIT 10
	`, tgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClipRequests(rows)
}

func (r *ClipRepo) ListPending() ([]*models.ClipRequest, error) {
	rows, err := r.db.Query(`
		SELECT cr.id, cr.client_tg_id, cr.client_name, cr.branch_id, cr.table_id,
		       cr.requested_time, cr.duration_sec, cr.status,
		       COALESCE(cr.clip_path,''), COALESCE(cr.notes,''), cr.created_at,
		       b.name, t.table_num
		FROM clip_requests cr
		JOIN branches b ON b.id = cr.branch_id
		JOIN tables t ON t.id = cr.table_id
		WHERE cr.status IN ('pending','paid')
		ORDER BY cr.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanClipRequests(rows)
}

func (r *ClipRepo) CreatePayment(clipRequestID int64, screenshotID string) error {
	_, err := r.db.Exec(`
		INSERT INTO payments (clip_request_id, amount, method, status, screenshot_id, paid_at)
		VALUES ($1, $2, 'manual', 'pending', $3, NULL)
	`, clipRequestID, models.ClipPrice, screenshotID)
	return err
}

func (r *ClipRepo) ConfirmPayment(clipRequestID int64) error {
	now := time.Now()
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`UPDATE payments SET status='paid', paid_at=$1 WHERE clip_request_id=$2`,
		now, clipRequestID,
	)
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		`UPDATE clip_requests SET status='paid' WHERE id=$1`,
		clipRequestID,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func scanClipRequests(rows *sql.Rows) ([]*models.ClipRequest, error) {
	var list []*models.ClipRequest
	for rows.Next() {
		cr := &models.ClipRequest{}
		if err := rows.Scan(
			&cr.ID, &cr.ClientTgID, &cr.ClientName, &cr.BranchID, &cr.TableID,
			&cr.RequestedTime, &cr.DurationSec, &cr.Status, &cr.ClipPath,
			&cr.Notes, &cr.CreatedAt, &cr.BranchName, &cr.TableNum,
		); err != nil {
			return nil, err
		}
		list = append(list, cr)
	}
	return list, rows.Err()
}
