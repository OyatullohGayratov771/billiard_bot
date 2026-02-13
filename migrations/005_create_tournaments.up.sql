CREATE TABLE tournaments (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    branch_id INT NOT NULL REFERENCES branches(id),
    max_players INT NOT NULL,
    status TEXT NOT NULL CHECK (
        status IN ('registration', 'ongoing', 'finished')
    ),
    created_by INT NOT NULL REFERENCES admins(id),
    created_at TIMESTAMP DEFAULT now()
);
