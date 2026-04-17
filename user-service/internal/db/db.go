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

	// Migration: phone ustunini qo'shish (agar yo'q bo'lsa)
	_, _ = db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS phone TEXT NOT NULL DEFAULT ''`)

	log.Println("✅ user-service: PostgreSQL ulandi")
	return db, nil
}
