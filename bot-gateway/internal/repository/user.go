package repository

import (
	"database/sql"
	"errors"

	"bot-gateway/internal/models"
)

var ErrNotFound = errors.New("topilmadi")

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Upsert(tgID int64, username, firstName, lastName string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(`
		INSERT INTO users (telegram_id, username, first_name, last_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (telegram_id) DO UPDATE
		  SET username   = EXCLUDED.username,
		      first_name = EXCLUDED.first_name,
		      last_name  = EXCLUDED.last_name
		RETURNING id, telegram_id, username, first_name, last_name,
		          role, branch_id, is_active, created_at
	`, tgID, username, firstName, lastName).Scan(
		&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.LastName,
		&u.Role, &u.BranchID, &u.IsActive, &u.CreatedAt,
	)
	return u, err
}

func (r *UserRepo) GetByTelegramID(tgID int64) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(`
		SELECT id, telegram_id, username, first_name, last_name,
		       role, branch_id, is_active, created_at
		FROM users WHERE telegram_id = $1
	`, tgID).Scan(
		&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.LastName,
		&u.Role, &u.BranchID, &u.IsActive, &u.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepo) GetByID(id int64) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(`
		SELECT id, telegram_id, username, first_name, last_name,
		       role, branch_id, is_active, created_at
		FROM users WHERE id = $1
	`, id).Scan(
		&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.LastName,
		&u.Role, &u.BranchID, &u.IsActive, &u.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (r *UserRepo) SetRole(tgID int64, role string, branchID *int64) error {
	_, err := r.db.Exec(
		`UPDATE users SET role = $1, branch_id = $2 WHERE telegram_id = $3`,
		role, branchID, tgID,
	)
	return err
}

func (r *UserRepo) SetActive(tgID int64, active bool) error {
	_, err := r.db.Exec(
		`UPDATE users SET is_active = $1 WHERE telegram_id = $2`,
		active, tgID,
	)
	return err
}

func (r *UserRepo) ListByRole(role string) ([]*models.User, error) {
	rows, err := r.db.Query(`
		SELECT id, telegram_id, username, first_name, last_name,
		       role, branch_id, is_active, created_at
		FROM users WHERE role = $1 ORDER BY created_at DESC
	`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func (r *UserRepo) ListAll() ([]*models.User, error) {
	rows, err := r.db.Query(`
		SELECT id, telegram_id, username, first_name, last_name,
		       role, branch_id, is_active, created_at
		FROM users ORDER BY role, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUsers(rows)
}

func scanUsers(rows *sql.Rows) ([]*models.User, error) {
	var users []*models.User
	for rows.Next() {
		u := &models.User{}
		if err := rows.Scan(
			&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.LastName,
			&u.Role, &u.BranchID, &u.IsActive, &u.CreatedAt,
		); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
