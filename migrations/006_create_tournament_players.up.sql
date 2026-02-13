CREATE TABLE tournament_players (
    tournament_id INT NOT NULL REFERENCES tournaments(id) ON DELETE CASCADE,
    player_id INT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    joined_at TIMESTAMP DEFAULT now(),
    PRIMARY KEY (tournament_id, player_id)
);
