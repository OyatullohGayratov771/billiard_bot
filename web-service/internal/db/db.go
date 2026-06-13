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

		CREATE TABLE IF NOT EXISTS products (
			id          BIGSERIAL PRIMARY KEY,
			name        TEXT        NOT NULL,
			description TEXT        NOT NULL DEFAULT '',
			price       BIGINT      NOT NULL DEFAULT 0,
			category    TEXT        NOT NULL DEFAULT '',
			emoji       TEXT        NOT NULL DEFAULT '🎱',
			image_url   TEXT        NOT NULL DEFAULT '',
			in_stock    BOOLEAN     NOT NULL DEFAULT true,
			sort_order  INT         NOT NULL DEFAULT 0,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_products_stock ON products(in_stock, sort_order);
	`); err != nil {
		return nil, err
	}
	return db, nil
}
