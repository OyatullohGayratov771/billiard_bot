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

	if err := alterMigrate(db); err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

func alterMigrate(db *sql.DB) error {
	_, err := db.Exec(`
		ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS join_code TEXT NOT NULL DEFAULT '';
		ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS type TEXT NOT NULL DEFAULT 'single_elimination';
		ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS table_count INT NOT NULL DEFAULT 0;
		ALTER TABLE tournaments ADD COLUMN IF NOT EXISTS slot_minutes INT NOT NULL DEFAULT 30;
		ALTER TABLE tournament_matches ADD COLUMN IF NOT EXISTS match_type TEXT NOT NULL DEFAULT 'winners';
		ALTER TABLE tournament_matches ADD COLUMN IF NOT EXISTS loser_next_match_id BIGINT;
		ALTER TABLE tournament_matches ADD COLUMN IF NOT EXISTS winner_next_match_id BIGINT;
		ALTER TABLE tournament_matches ADD COLUMN IF NOT EXISTS table_num INT NOT NULL DEFAULT 0;
		ALTER TABLE tournament_matches ADD COLUMN IF NOT EXISTS player1_score INT NOT NULL DEFAULT 0;
		ALTER TABLE tournament_matches ADD COLUMN IF NOT EXISTS player2_score INT NOT NULL DEFAULT 0;
		ALTER TABLE tournament_matches ADD COLUMN IF NOT EXISTS match_scheduled_at TIMESTAMPTZ;
	`)
	if err != nil {
		log.Printf("❌ alterMigrate xatosi: %v", err)
	}
	return err
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

		CREATE TABLE IF NOT EXISTS tv_tokens (
			token         TEXT    PRIMARY KEY,
			tournament_id BIGINT  NOT NULL,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		log.Printf("❌ Migration xatosi: %v", err)
	} else {
		log.Println("✅ tournament-service: Jadvallar tayyor")
	}
	return err
}
