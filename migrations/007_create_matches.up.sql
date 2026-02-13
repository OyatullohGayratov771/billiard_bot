CREATE TABLE matches (
    id SERIAL PRIMARY KEY,
    tournament_id INT NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    player1_id INT NOT NULL REFERENCES players(id),
    player2_id INT NOT NULL REFERENCES players(id),
    winner_id INT REFERENCES players(id),
    round INT NOT NULL,
    created_at TIMESTAMP DEFAULT now()
);
