package repository

import (
	"database/sql"
	"errors"

	"bot-gateway/internal/models"
)

type BranchRepo struct {
	db *sql.DB
}

func NewBranchRepo(db *sql.DB) *BranchRepo {
	return &BranchRepo{db: db}
}

func (r *BranchRepo) GetAll() ([]*models.Branch, error) {
	rows, err := r.db.Query(`
		SELECT id, name, address, COALESCE(nvr_ip,''), nvr_port,
		       COALESCE(nvr_user,''), COALESCE(nvr_pass,''), created_at
		FROM branches ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var branches []*models.Branch
	for rows.Next() {
		b := &models.Branch{}
		if err := rows.Scan(
			&b.ID, &b.Name, &b.Address, &b.NVRHost, &b.NVRPort,
			&b.NVRUser, &b.NVRPass, &b.CreatedAt,
		); err != nil {
			return nil, err
		}
		branches = append(branches, b)
	}
	return branches, rows.Err()
}

func (r *BranchRepo) GetByID(id int64) (*models.Branch, error) {
	b := &models.Branch{}
	err := r.db.QueryRow(`
		SELECT id, name, address, COALESCE(nvr_ip,''), nvr_port,
		       COALESCE(nvr_user,''), COALESCE(nvr_pass,''), created_at
		FROM branches WHERE id = $1
	`, id).Scan(
		&b.ID, &b.Name, &b.Address, &b.NVRHost, &b.NVRPort,
		&b.NVRUser, &b.NVRPass, &b.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

func (r *BranchRepo) UpdateNVR(id int64, ip string, port int, user, pass string) error {
	_, err := r.db.Exec(
		`UPDATE branches SET nvr_ip=$1, nvr_port=$2, nvr_user=$3, nvr_pass=$4 WHERE id=$5`,
		ip, port, user, pass, id,
	)
	return err
}
