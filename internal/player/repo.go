package player

import "github.com/jmoiron/sqlx"

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Telegram ID bo‘yicha topamiz
func (r *Repository) GetByTelegramID(tgID int64) (*Player, error) {
	var p Player
	err := r.db.Get(&p,
		`SELECT id, telegram_id, name, rating
		FROM players
		WHERE telegram_id=$1
		`,
		tgID,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Yangi player yaratamiz
func (r *Repository) Create(tgID int64, name string) error {
	_, err := r.db.Exec(
		`INSERT INTO players (telegram_id, name)
		 VALUES ($1, $2)`,
		tgID, name,
	)
	return err
}
