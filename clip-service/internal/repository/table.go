package repository

import (
	"database/sql"
	"errors"

	"clip-service/internal/models"
)

type TableRepo struct {
	db *sql.DB
}

func NewTableRepo(db *sql.DB) *TableRepo {
	return &TableRepo{db: db}
}

func (r *TableRepo) GetByID(id int64) (*models.Table, error) {
	t := &models.Table{}
	err := r.db.QueryRow(`
		SELECT id, branch_id, table_num, COALESCE(camera_channel, 0), COALESCE(rtsp_url, '')
		FROM tables WHERE id = $1
	`, id).Scan(&t.ID, &t.BranchID, &t.TableNum, &t.CameraChannel, &t.RTSPUrl)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}
