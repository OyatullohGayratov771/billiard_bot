package db

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"
)

// New — live-service faqat boshqa servislar yaratgan branches/tables
// jadvallarini o'qiydi, shuning uchun bu yerda migration yo'q.
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
	log.Println("✅ live-service: PostgreSQL ulandi")
	return db, nil
}
