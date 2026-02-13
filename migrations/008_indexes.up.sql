CREATE INDEX idx_admins_telegram_id ON admins(telegram_id);
CREATE INDEX idx_players_telegram_id ON players(telegram_id);
CREATE INDEX idx_tournaments_status ON tournaments(status);
CREATE INDEX idx_matches_tournament_id ON matches(tournament_id);
