package repository

import (
	"database/sql"
	"errors"

	"clip-service/internal/models"
)

type BranchRepo struct {
	db *sql.DB
}

func NewBranchRepo(db *sql.DB) *BranchRepo {
	return &BranchRepo{db: db}
}

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
