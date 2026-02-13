CREATE TABLE branches (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    address TEXT,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT now()
);
