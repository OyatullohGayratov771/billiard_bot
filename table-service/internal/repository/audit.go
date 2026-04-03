package repository

import "database/sql"

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
