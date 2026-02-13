package auth

type Admin struct {
	ID         int   `db:"id"`
	TelegramID int64 `db:"telegram_id"`
	Role       string `db:"role"`
}
