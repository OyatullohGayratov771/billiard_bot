package player

import "time"

type Player struct {
	ID         int       `db:"id"`
	TelegramID int64     `db:"telegram_id"`
	Name       string    `db:"name"`
	Rating     int       `db:"rating"`
	CreatedAt  time.Time `db:"created_at"`
}
