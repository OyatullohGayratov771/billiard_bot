package db

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

func New(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	log.Println("✅ web-service: PostgreSQL ulandi")

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS web_tokens (
			token      TEXT PRIMARY KEY,
			tg_id      BIGINT NOT NULL,
			role       TEXT NOT NULL DEFAULT 'client',
			expires_at TIMESTAMPTZ NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_web_tokens_expires ON web_tokens(expires_at);
		ALTER TABLE users ADD COLUMN IF NOT EXISTS name_customized BOOLEAN NOT NULL DEFAULT false;
	`); err != nil {
		return nil, err
	}
	return db, nil
}
