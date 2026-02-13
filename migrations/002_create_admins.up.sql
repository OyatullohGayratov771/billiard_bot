CREATE TABLE admins (
    id SERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE NOT NULL,
    name TEXT,
    role TEXT NOT NULL CHECK (role IN ('superadmin', 'admin')),
    created_at TIMESTAMP DEFAULT now()
);
