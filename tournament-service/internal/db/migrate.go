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
	log.Println("✅ tournament-service: PostgreSQL ulandi")

	if err := migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tournaments (
			id           BIGSERIAL PRIMARY KEY,
			name         TEXT        NOT NULL,
			branch_id    BIGINT      NOT NULL,
			table_id     BIGINT,
			scheduled_at TIMESTAMPTZ NOT NULL,
			price        BIGINT      NOT NULL DEFAULT 0,
			max_players  INT         NOT NULL DEFAULT 8,
			status       TEXT        NOT NULL DEFAULT 'registration',
			created_by   BIGINT      NOT NULL,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS tournament_registrations (
			id            BIGSERIAL PRIMARY KEY,
			tournament_id BIGINT      NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
			user_tg_id    BIGINT      NOT NULL,
			user_name     TEXT        NOT NULL,
			status        TEXT        NOT NULL DEFAULT 'pending',
			registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			decided_at    TIMESTAMPTZ,
			UNIQUE(tournament_id, user_tg_id)
		);

		CREATE TABLE IF NOT EXISTS tournament_matches (
			id            BIGSERIAL PRIMARY KEY,
			tournament_id BIGINT NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
			round         INT    NOT NULL,
			match_num     INT    NOT NULL,
			player1_tg_id BIGINT,
			player2_tg_id BIGINT,
			winner_tg_id  BIGINT,
			status        TEXT   NOT NULL DEFAULT 'pending',
			UNIQUE(tournament_id, round, match_num)
		);
	`)
	if err != nil {
		log.Printf("❌ Migration xatosi: %v", err)
	} else {
		log.Println("✅ tournament-service: Jadvallar tayyor")
	}
	return err
}
