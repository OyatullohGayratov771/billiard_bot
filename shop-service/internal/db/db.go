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
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(3)
	log.Println("✅ shop-service: PostgreSQL ulandi")

	if _, err := db.Exec(`
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
	log.Println("✅ shop-service: Jadval tayyor")
	return db, nil
}
