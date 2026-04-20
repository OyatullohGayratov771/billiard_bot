package db

import (
	"database/sql"
	"log"
)

const schema = `
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

CREATE TABLE IF NOT EXISTS tables (
    id               SERIAL PRIMARY KEY,
    branch_id        INT NOT NULL REFERENCES branches(id),
    table_num        INT NOT NULL,
    camera_channel   INT,
    status           VARCHAR(20) DEFAULT 'free',
    price_per_hour   BIGINT DEFAULT 20000,
    created_at       TIMESTAMP DEFAULT NOW(),
    UNIQUE(branch_id, table_num)
);

CREATE TABLE IF NOT EXISTS users (
    id           SERIAL PRIMARY KEY,
    telegram_id  BIGINT UNIQUE NOT NULL,
    username     VARCHAR(100),
    first_name   VARCHAR(100),
    last_name    VARCHAR(100),
    role         VARCHAR(20) DEFAULT 'client',
    branch_id    INT REFERENCES branches(id),
    is_active    BOOLEAN DEFAULT TRUE,
    created_at   TIMESTAMP DEFAULT NOW()
);

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

CREATE TABLE IF NOT EXISTS clip_requests (
    id              SERIAL PRIMARY KEY,
    client_tg_id    BIGINT NOT NULL,
    client_name     VARCHAR(100),
    branch_id       INT NOT NULL REFERENCES branches(id),
    table_id        INT NOT NULL REFERENCES tables(id),
    start_time      TIMESTAMPTZ,
    end_time        TIMESTAMPTZ,
    status          VARCHAR(20) DEFAULT 'pending',
    clip_path       TEXT,
    notes           TEXT,
    created_at      TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payments (
    id               SERIAL PRIMARY KEY,
    clip_request_id  INT NOT NULL REFERENCES clip_requests(id),
    amount           BIGINT NOT NULL DEFAULT 1000000,
    method           VARCHAR(20) DEFAULT 'manual',
    status           VARCHAR(20) DEFAULT 'pending',
    provider_id      VARCHAR(200),
    screenshot_id    VARCHAR(200),
    paid_at          TIMESTAMP,
    created_at       TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_log (
    id          SERIAL PRIMARY KEY,
    user_id     INT REFERENCES users(id),
    action      VARCHAR(100) NOT NULL,
    details     TEXT,
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notifications (
    id          SERIAL PRIMARY KEY,
    type        VARCHAR(50),
    target_id   INT,
    message     TEXT,
    sent_at     TIMESTAMP DEFAULT NOW(),
    status      VARCHAR(20) DEFAULT 'sent'
);

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
	// Eski bazaga rtsp_url ustuni qo'shish
	_, _ = db.Exec(`ALTER TABLE tables ADD COLUMN IF NOT EXISTS rtsp_url TEXT`)
	// Eski bazaga start_time / end_time ustunlari qo'shish
	_, _ = db.Exec(`ALTER TABLE clip_requests ADD COLUMN IF NOT EXISTS start_time TIMESTAMPTZ`)
	_, _ = db.Exec(`ALTER TABLE clip_requests ADD COLUMN IF NOT EXISTS end_time   TIMESTAMPTZ`)
	// Eski NOT NULL cheklovini olib tashlash
	_, _ = db.Exec(`ALTER TABLE clip_requests ALTER COLUMN requested_time DROP NOT NULL`)
	log.Println("✅ Migration bajarildi (8 jadval)")
	return nil
}

func SeedBranches(db *sql.DB) error {
	type branchSeed struct {
		name     string
		address  string
		nvrIP    string
		nvrPort  int
		nvrUser  string
		nvrPass  string
		channels []int // table_num 1..N → camera_channel
	}

	branches := []branchSeed{
		{
			name:     "Kokcha",
			address:  "Toshkent, Chilonzor",
			nvrIP:    "192.168.68.107",
			nvrPort:  554,
			nvrUser:  "admin",
			nvrPass:  "QWERTY12",
			channels: []int{14, 12, 13, 7, 9, 16, 15, 10, 8},
		},
		{
			name:     "Toshmi",
			address:  "Toshkent, Yunusobod",
			channels: []int{1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
	}

	for _, b := range branches {
		var id int
		// Avval mavjudligini tekshiramiz
		err := db.QueryRow(`SELECT id FROM branches WHERE name = $1`, b.name).Scan(&id)
		if err == sql.ErrNoRows {
			// Yangi filial yaratamiz
			err = db.QueryRow(
				`INSERT INTO branches (name, address, nvr_ip, nvr_port, nvr_user, nvr_pass)
				 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
				b.name, b.address, b.nvrIP, b.nvrPort, b.nvrUser, b.nvrPass,
			).Scan(&id)
			if err != nil {
				return err
			}
			log.Printf("✅ %s: yangi filial yaratildi (id=%d)", b.name, id)
		} else if err != nil {
			return err
		}

		// Stollarni tekshirib yaratamiz (mavjud bo'lsa o'tkazib yuboramiz)
		created := 0
		for i, ch := range b.channels {
			tableNum := i + 1
			res, tErr := db.Exec(
				`INSERT INTO tables (branch_id, table_num, camera_channel, price_per_hour)
				 VALUES ($1, $2, $3, 2000000)
				 ON CONFLICT (branch_id, table_num) DO NOTHING`,
				id, tableNum, ch,
			)
			if tErr != nil {
				return tErr
			}
			if n, _ := res.RowsAffected(); n > 0 {
				created++
			}
		}
		if created > 0 {
			log.Printf("✅ %s: %d ta yangi stol yaratildi", b.name, created)
		} else {
			log.Printf("ℹ️  %s: stollar allaqachon mavjud, o'tkazildi", b.name)
		}
	}
	return nil
}
