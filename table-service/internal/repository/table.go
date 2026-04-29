package repository

import (
	"database/sql"
	"errors"

	"table-service/internal/models"
)

type TableRepo struct {
	db *sql.DB
}

func NewTableRepo(db *sql.DB) *TableRepo {
	return &TableRepo{db: db}
}

func (r *TableRepo) GetByBranch(branchID int64) ([]*models.Table, error) {
	rows, err := r.db.Query(`
		SELECT t.id, t.branch_id, t.table_num, COALESCE(t.camera_channel,0),
		       COALESCE(t.rtsp_url,''), t.status, t.price_per_hour, t.created_at, b.name
		FROM tables t
		JOIN branches b ON b.id = t.branch_id
		WHERE t.branch_id = $1
		ORDER BY t.table_num
	`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTables(rows)
}

func (r *TableRepo) GetByID(id int64) (*models.Table, error) {
	t := &models.Table{}
	err := r.db.QueryRow(`
		SELECT t.id, t.branch_id, t.table_num, COALESCE(t.camera_channel,0),
		       COALESCE(t.rtsp_url,''), t.status, t.price_per_hour, t.created_at, b.name
		FROM tables t
		JOIN branches b ON b.id = t.branch_id
		WHERE t.id = $1
	`, id).Scan(
		&t.ID, &t.BranchID, &t.TableNum, &t.CameraChannel, &t.RTSPUrl,
		&t.Status, &t.PricePerHour, &t.CreatedAt, &t.BranchName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func (r *TableRepo) SetStatus(id int64, status string) error {
	_, err := r.db.Exec(`UPDATE tables SET status = $1 WHERE id = $2`, status, id)
	return err
}

func (r *TableRepo) SetRTSPUrl(id int64, rtspURL string) error {
	_, err := r.db.Exec(`UPDATE tables SET rtsp_url = $1 WHERE id = $2`, rtspURL, id)
	return err
}

func (r *TableRepo) SetCameraChannel(id int64, channel int) error {
	_, err := r.db.Exec(`UPDATE tables SET camera_channel = $1 WHERE id = $2`, channel, id)
	return err
}

func scanTables(rows *sql.Rows) ([]*models.Table, error) {
	var tables []*models.Table
	for rows.Next() {
		t := &models.Table{}
		if err := rows.Scan(
			&t.ID, &t.BranchID, &t.TableNum, &t.CameraChannel, &t.RTSPUrl,
			&t.Status, &t.PricePerHour, &t.CreatedAt, &t.BranchName,
		); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}
