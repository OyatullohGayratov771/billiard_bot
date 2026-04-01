package repository

import (
	"database/sql"

	"bot-gateway/internal/models"
)

type AuditRepo struct {
	db *sql.DB
}

func NewAuditRepo(db *sql.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

func (r *AuditRepo) Log(userID *int64, action, details string) {
	_, _ = r.db.Exec(
		`INSERT INTO audit_log (user_id, action, details) VALUES ($1, $2, $3)`,
		userID, action, details,
	)
}

func (r *AuditRepo) Recent(limit int) ([]*models.AuditLog, error) {
	rows, err := r.db.Query(`
		SELECT id, user_id, action, details, created_at
		FROM audit_log
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		l := &models.AuditLog{}
		if err := rows.Scan(&l.ID, &l.UserID, &l.Action, &l.Details, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
