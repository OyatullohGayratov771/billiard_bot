CREATE TABLE players (
    id SERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    rating INT DEFAULT 1000,
    created_at TIMESTAMP DEFAULT now()
);
