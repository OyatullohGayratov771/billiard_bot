package auth

import "github.com/jmoiron/sqlx"

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetByTelegramID(tgID int64) (*Admin, error) {
	var a Admin
	err := r.db.Get(&a,
		`SELECT * FROM admins WHERE telegram_id=$1`,
		tgID,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
