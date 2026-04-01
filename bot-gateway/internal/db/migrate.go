package db

import (
	"database/sql"
	"log"
)

const schema = `
-- Filiallar
CREATE TABLE IF NOT EXISTS branches (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(100) NOT NULL,
    address         TEXT,
    nvr_ip          VARCHAR(50),
    nvr_port        INT DEFAULT 80,
    nvr_user        VARCHAR(100),
    nvr_pass        VARCHAR(100),
    created_at      TIMESTAMP DEFAULT NOW()
);

-- Stollar
CREATE TABLE IF NOT EXISTS tables (
    id               SERIAL PRIMARY KEY,
    branch_id        INT NOT NULL REFERENCES branches(id),
    table_num        INT NOT NULL,
    camera_channel   INT,
    status           VARCHAR(20) DEFAULT 'free',   -- free | busy
    price_per_hour   BIGINT DEFAULT 20000,         -- tiyinda (som * 100)
    created_at       TIMESTAMP DEFAULT NOW(),
    UNIQUE(branch_id, table_num)
);

-- Bot foydalanuvchilari (admin/operator)
CREATE TABLE IF NOT EXISTS users (
    id           SERIAL PRIMARY KEY,
    telegram_id  BIGINT UNIQUE NOT NULL,
    username     VARCHAR(100),
    first_name   VARCHAR(100),
    last_name    VARCHAR(100),
    role         VARCHAR(20) DEFAULT 'client',  -- superadmin | admin | operator | client
    branch_id    INT REFERENCES branches(id),
    is_active    BOOLEAN DEFAULT TRUE,
    created_at   TIMESTAMP DEFAULT NOW()
);

-- O'yin sessiyalari
CREATE TABLE IF NOT EXISTS sessions (
    id           SERIAL PRIMARY KEY,
    table_id     INT NOT NULL REFERENCES tables(id),
    operator_id  INT NOT NULL REFERENCES users(id),
    client_name  VARCHAR(100),
    started_at   TIMESTAMP NOT NULL DEFAULT NOW(),
    ended_at     TIMESTAMP,
    total_min    INT DEFAULT 0,
    total_price  BIGINT DEFAULT 0,
    created_at   TIMESTAMP DEFAULT NOW()
);

-- Klip buyurtmalari
CREATE TABLE IF NOT EXISTS clip_requests (
    id              SERIAL PRIMARY KEY,
    client_tg_id    BIGINT NOT NULL,
    client_name     VARCHAR(100),
    branch_id       INT NOT NULL REFERENCES branches(id),
    table_id        INT NOT NULL REFERENCES tables(id),
    requested_time  TIMESTAMP NOT NULL,
    duration_sec    INT DEFAULT 60,
    status          VARCHAR(20) DEFAULT 'pending',  -- pending|paid|processing|done|failed|refunded
    clip_path       TEXT,
    notes           TEXT,
    created_at      TIMESTAMP DEFAULT NOW()
);

-- To'lovlar
CREATE TABLE IF NOT EXISTS payments (
    id               SERIAL PRIMARY KEY,
    clip_request_id  INT NOT NULL REFERENCES clip_requests(id),
    amount           BIGINT NOT NULL DEFAULT 1000000,  -- 10,000 som = 1,000,000 tiyin
    method           VARCHAR(20) DEFAULT 'manual',     -- manual | click | payme
    status           VARCHAR(20) DEFAULT 'pending',    -- pending | paid | refunded
    provider_id      VARCHAR(200),
    screenshot_id    VARCHAR(200),
    paid_at          TIMESTAMP,
    created_at       TIMESTAMP DEFAULT NOW()
);

-- Audit log
CREATE TABLE IF NOT EXISTS audit_log (
    id          SERIAL PRIMARY KEY,
    user_id     INT REFERENCES users(id),
    action      VARCHAR(100) NOT NULL,
    details     TEXT,
    created_at  TIMESTAMP DEFAULT NOW()
);

-- Bildirishnomalar
CREATE TABLE IF NOT EXISTS notifications (
    id          SERIAL PRIMARY KEY,
    type        VARCHAR(50),
    target_id   INT,
    message     TEXT,
    sent_at     TIMESTAMP DEFAULT NOW(),
    status      VARCHAR(20) DEFAULT 'sent'
);

-- Indekslar
CREATE INDEX IF NOT EXISTS idx_tables_branch ON tables(branch_id);
CREATE INDEX IF NOT EXISTS idx_tables_status ON tables(status);
CREATE INDEX IF NOT EXISTS idx_sessions_table ON sessions(table_id);
CREATE INDEX IF NOT EXISTS idx_sessions_ended ON sessions(ended_at);
CREATE INDEX IF NOT EXISTS idx_clip_requests_client ON clip_requests(client_tg_id);
CREATE INDEX IF NOT EXISTS idx_clip_requests_status ON clip_requests(status);
CREATE INDEX IF NOT EXISTS idx_audit_log_user ON audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log(created_at);
`

func Migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	if err != nil {
		return err
	}
	log.Println("✅ Migration bajarildi (8 jadval)")
	return nil
}

// SeedBranches — ishga tushganda 2 ta standart filial qo'shadi (agar yo'q bo'lsa)
func SeedBranches(db *sql.DB) error {
	branches := []struct {
		name    string
		address string
		tables  int
	}{
		{"Filial 1", "Toshkent, Chilonzor", 9},
		{"Filial 2", "Toshkent, Yunusobod", 9},
	}

	for _, b := range branches {
		var id int
		err := db.QueryRow(
			`INSERT INTO branches (name, address) VALUES ($1, $2)
			 ON CONFLICT DO NOTHING RETURNING id`,
			b.name, b.address,
		).Scan(&id)

		if err == sql.ErrNoRows {
			// already exists
			continue
		}
		if err != nil {
			return err
		}

		// stollar qo'sh
		for i := 1; i <= b.tables; i++ {
			_, err := db.Exec(
				`INSERT INTO tables (branch_id, table_num, camera_channel, price_per_hour)
				 VALUES ($1, $2, $3, 2000000)
				 ON CONFLICT DO NOTHING`,
				id, i, i,
			)
			if err != nil {
				return err
			}
		}
		log.Printf("✅ %s: %d ta stol yaratildi", b.name, b.tables)
	}
	return nil
}
