package repository

import (
	"database/sql"
	"errors"

	"bot-gateway/internal/models"
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
		       t.status, t.price_per_hour, t.created_at, b.name
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

func (r *TableRepo) GetAll() ([]*models.Table, error) {
	rows, err := r.db.Query(`
		SELECT t.id, t.branch_id, t.table_num, COALESCE(t.camera_channel,0),
		       t.status, t.price_per_hour, t.created_at, b.name
		FROM tables t
		JOIN branches b ON b.id = t.branch_id
		ORDER BY t.branch_id, t.table_num
	`)
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
		       t.status, t.price_per_hour, t.created_at, b.name
		FROM tables t
		JOIN branches b ON b.id = t.branch_id
		WHERE t.id = $1
	`, id).Scan(
		&t.ID, &t.BranchID, &t.TableNum, &t.CameraChannel,
		&t.Status, &t.PricePerHour, &t.CreatedAt, &t.BranchName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func (r *TableRepo) SetStatus(id int64, status string) error {
	_, err := r.db.Exec(
		`UPDATE tables SET status = $1 WHERE id = $2`,
		status, id,
	)
	return err
}

func (r *TableRepo) SetPrice(id int64, pricePerHour int64) error {
	_, err := r.db.Exec(
		`UPDATE tables SET price_per_hour = $1 WHERE id = $2`,
		pricePerHour, id,
	)
	return err
}

func scanTables(rows *sql.Rows) ([]*models.Table, error) {
	var tables []*models.Table
	for rows.Next() {
		t := &models.Table{}
		if err := rows.Scan(
			&t.ID, &t.BranchID, &t.TableNum, &t.CameraChannel,
			&t.Status, &t.PricePerHour, &t.CreatedAt, &t.BranchName,
		); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}
