package repository

import (
	"database/sql"
	"errors"

	"live-service/internal/models"
)

var ErrNotFound = errors.New("topilmadi")

type BranchRepo struct{ db *sql.DB }

func NewBranchRepo(db *sql.DB) *BranchRepo { return &BranchRepo{db: db} }

func (r *BranchRepo) GetByID(id int64) (*models.Branch, error) {
	b := &models.Branch{}
	err := r.db.QueryRow(`
		SELECT id, name, COALESCE(nvr_ip,''), nvr_port,
		       COALESCE(nvr_user,''), COALESCE(nvr_pass,'')
		FROM branches WHERE id = $1
	`, id).Scan(&b.ID, &b.Name, &b.NVRHost, &b.NVRPort, &b.NVRUser, &b.NVRPass)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

type TableRepo struct{ db *sql.DB }

func NewTableRepo(db *sql.DB) *TableRepo { return &TableRepo{db: db} }

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

func (r *TableRepo) ListByBranch(branchID int64) ([]*models.Table, error) {
	rows, err := r.db.Query(`
		SELECT id, branch_id, table_num, COALESCE(camera_channel, 0), COALESCE(rtsp_url, '')
		FROM tables WHERE branch_id = $1 ORDER BY table_num
	`, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]*models.Table, 0)
	for rows.Next() {
		t := &models.Table{}
		if err := rows.Scan(&t.ID, &t.BranchID, &t.TableNum, &t.CameraChannel, &t.RTSPUrl); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}
