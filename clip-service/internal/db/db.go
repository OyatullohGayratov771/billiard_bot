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
	log.Println("✅ clip-service: PostgreSQL ulandi")

	// clip_requests jadvaliga start_time / end_time ustunlarini qo'shish
	// (eski requested_time + duration_sec o'rniga)
	_, _ = db.Exec(`ALTER TABLE clip_requests ADD COLUMN IF NOT EXISTS start_time TIMESTAMPTZ`)
	_, _ = db.Exec(`ALTER TABLE clip_requests ADD COLUMN IF NOT EXISTS end_time   TIMESTAMPTZ`)

	return db, nil
}
