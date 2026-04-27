package repository

import (
	"database/sql"
	"errors"
	"time"

	"tournament-service/internal/models"
)

var ErrNotFound = errors.New("topilmadi")

type Repo struct {
	db *sql.DB
}

func New(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// ===================== TOURNAMENTS =====================

func (r *Repo) CreateTournament(name string, branchID int64, tableID *int64, scheduledAt time.Time, price int64, maxPlayers int, createdBy int64) (*models.Tournament, error) {
	t := &models.Tournament{}
	err := r.db.QueryRow(`
		INSERT INTO tournaments (name, branch_id, table_id, scheduled_at, price, max_players, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, name, branch_id, table_id, scheduled_at, price, max_players, status, created_by, created_at`,
		name, branchID, tableID, scheduledAt, price, maxPlayers, createdBy,
	).Scan(&t.ID, &t.Name, &t.BranchID, &t.TableID, &t.ScheduledAt, &t.Price, &t.MaxPlayers, &t.Status, &t.CreatedBy, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	r.fillTournamentJoins(t)
	return t, nil
}

func (r *Repo) GetTournament(id int64) (*models.Tournament, error) {
	t := &models.Tournament{}
	err := r.db.QueryRow(`
		SELECT id, name, branch_id, table_id, scheduled_at, price, max_players, status, created_by, created_at
		FROM tournaments WHERE id=$1`, id,
	).Scan(&t.ID, &t.Name, &t.BranchID, &t.TableID, &t.ScheduledAt, &t.Price, &t.MaxPlayers, &t.Status, &t.CreatedBy, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.fillTournamentJoins(t)
	return t, nil
}

func (r *Repo) ListTournaments(status string) ([]*models.Tournament, error) {
	q := `SELECT id, name, branch_id, table_id, scheduled_at, price, max_players, status, created_by, created_at
		  FROM tournaments`
	var args []any
	if status != "" {
		q += " WHERE status=$1"
		args = append(args, status)
	}
	q += " ORDER BY scheduled_at ASC"

	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.Tournament
	for rows.Next() {
		t := &models.Tournament{}
		if err := rows.Scan(&t.ID, &t.Name, &t.BranchID, &t.TableID, &t.ScheduledAt, &t.Price, &t.MaxPlayers, &t.Status, &t.CreatedBy, &t.CreatedAt); err != nil {
			return nil, err
		}
		r.fillTournamentJoins(t)
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *Repo) SetTournamentStatus(id int64, status string) error {
	_, err := r.db.Exec(`UPDATE tournaments SET status=$1 WHERE id=$2`, status, id)
	return err
}

func (r *Repo) fillTournamentJoins(t *models.Tournament) {
	_ = r.db.QueryRow(`SELECT name FROM branches WHERE id=$1`, t.BranchID).Scan(&t.BranchName)
	if t.TableID != nil {
		_ = r.db.QueryRow(`SELECT table_num FROM tables WHERE id=$1`, *t.TableID).Scan(&t.TableNum)
	}
	_ = r.db.QueryRow(`SELECT COUNT(*) FROM tournament_registrations WHERE tournament_id=$1 AND status='approved'`, t.ID).Scan(&t.ApprovedCount)
}

// ===================== REGISTRATIONS =====================

func (r *Repo) Register(tournamentID, userTgID int64, userName string) (*models.Registration, error) {
	reg := &models.Registration{}
	err := r.db.QueryRow(`
		INSERT INTO tournament_registrations (tournament_id, user_tg_id, user_name)
		VALUES ($1,$2,$3)
		ON CONFLICT (tournament_id, user_tg_id) DO NOTHING
		RETURNING id, tournament_id, user_tg_id, user_name, status, registered_at, decided_at`,
		tournamentID, userTgID, userName,
	).Scan(&reg.ID, &reg.TournamentID, &reg.UserTgID, &reg.UserName, &reg.Status, &reg.RegisteredAt, &reg.DecidedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("siz allaqachon ro'yxatdan o'tgansiz")
	}
	return reg, err
}


func (r *Repo) GetRegistration(id int64) (*models.Registration, error) {
	reg := &models.Registration{}
	err := r.db.QueryRow(`
		SELECT id, tournament_id, user_tg_id, user_name, status, registered_at, decided_at
		FROM tournament_registrations WHERE id=$1`, id,
	).Scan(&reg.ID, &reg.TournamentID, &reg.UserTgID, &reg.UserName, &reg.Status, &reg.RegisteredAt, &reg.DecidedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return reg, err
}

func (r *Repo) ListRegistrations(tournamentID int64) ([]*models.Registration, error) {
	rows, err := r.db.Query(`
		SELECT id, tournament_id, user_tg_id, user_name, status, registered_at, decided_at
		FROM tournament_registrations WHERE tournament_id=$1 ORDER BY registered_at ASC`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Registration
	for rows.Next() {
		reg := &models.Registration{}
		if err := rows.Scan(&reg.ID, &reg.TournamentID, &reg.UserTgID, &reg.UserName, &reg.Status, &reg.RegisteredAt, &reg.DecidedAt); err != nil {
			return nil, err
		}
		list = append(list, reg)
	}
	return list, rows.Err()
}

func (r *Repo) ListApprovedRegistrations(tournamentID int64) ([]*models.Registration, error) {
	rows, err := r.db.Query(`
		SELECT id, tournament_id, user_tg_id, user_name, status, registered_at, decided_at
		FROM tournament_registrations WHERE tournament_id=$1 AND status='approved' ORDER BY registered_at ASC`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Registration
	for rows.Next() {
		reg := &models.Registration{}
		if err := rows.Scan(&reg.ID, &reg.TournamentID, &reg.UserTgID, &reg.UserName, &reg.Status, &reg.RegisteredAt, &reg.DecidedAt); err != nil {
			return nil, err
		}
		list = append(list, reg)
	}
	return list, rows.Err()
}

func (r *Repo) SetRegistrationStatus(id int64, status string) error {
	now := time.Now()
	_, err := r.db.Exec(`UPDATE tournament_registrations SET status=$1, decided_at=$2 WHERE id=$3`, status, now, id)
	return err
}

func (r *Repo) GetUserRegistration(tournamentID, userTgID int64) (*models.Registration, error) {
	reg := &models.Registration{}
	err := r.db.QueryRow(`
		SELECT id, tournament_id, user_tg_id, user_name, status, registered_at, decided_at
		FROM tournament_registrations WHERE tournament_id=$1 AND user_tg_id=$2`,
		tournamentID, userTgID,
	).Scan(&reg.ID, &reg.TournamentID, &reg.UserTgID, &reg.UserName, &reg.Status, &reg.RegisteredAt, &reg.DecidedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return reg, err
}

func (r *Repo) GetUserTournaments(userTgID int64) ([]*models.Registration, error) {
	rows, err := r.db.Query(`
		SELECT tr.id, tr.tournament_id, tr.user_tg_id, tr.user_name, tr.status, tr.registered_at, tr.decided_at,
		       COALESCE(t.name, '')
		FROM tournament_registrations tr
		LEFT JOIN tournaments t ON t.id = tr.tournament_id
		WHERE tr.user_tg_id=$1 ORDER BY tr.registered_at DESC LIMIT 20`, userTgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Registration
	for rows.Next() {
		reg := &models.Registration{}
		if err := rows.Scan(&reg.ID, &reg.TournamentID, &reg.UserTgID, &reg.UserName, &reg.Status, &reg.RegisteredAt, &reg.DecidedAt, &reg.TournamentName); err != nil {
			return nil, err
		}
		list = append(list, reg)
	}
	return list, rows.Err()
}

// ===================== MATCHES =====================

func (r *Repo) InsertMatch(tx *sql.Tx, tournamentID int64, round, matchNum int) (int64, error) {
	var id int64
	err := tx.QueryRow(`
		INSERT INTO tournament_matches (tournament_id, round, match_num, status)
		VALUES ($1,$2,$3,'pending') RETURNING id`,
		tournamentID, round, matchNum,
	).Scan(&id)
	return id, err
}

func (r *Repo) GetMatchByRoundNum(tx *sql.Tx, tournamentID int64, round, matchNum int) (*models.Match, error) {
	m := &models.Match{}
	var row *sql.Row
	if tx != nil {
		row = tx.QueryRow(`
			SELECT id, tournament_id, round, match_num,
			       player1_tg_id, player2_tg_id, winner_tg_id, status
			FROM tournament_matches WHERE tournament_id=$1 AND round=$2 AND match_num=$3`,
			tournamentID, round, matchNum)
	} else {
		row = r.db.QueryRow(`
			SELECT id, tournament_id, round, match_num,
			       player1_tg_id, player2_tg_id, winner_tg_id, status
			FROM tournament_matches WHERE tournament_id=$1 AND round=$2 AND match_num=$3`,
			tournamentID, round, matchNum)
	}
	err := row.Scan(&m.ID, &m.TournamentID, &m.Round, &m.MatchNum, &m.Player1TgID, &m.Player2TgID, &m.WinnerTgID, &m.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

func (r *Repo) GetMatch(id int64) (*models.Match, error) {
	m := &models.Match{}
	err := r.db.QueryRow(`
		SELECT id, tournament_id, round, match_num,
		       player1_tg_id, player2_tg_id, winner_tg_id, status
		FROM tournament_matches WHERE id=$1`, id,
	).Scan(&m.ID, &m.TournamentID, &m.Round, &m.MatchNum, &m.Player1TgID, &m.Player2TgID, &m.WinnerTgID, &m.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

func (r *Repo) UpdateMatchPlayers(tx *sql.Tx, id int64, p1, p2 *int64, status string) error {
	_, err := tx.Exec(`UPDATE tournament_matches SET player1_tg_id=$1, player2_tg_id=$2, status=$3 WHERE id=$4`, p1, p2, status, id)
	return err
}

func (r *Repo) UpdateMatchStatus(tx *sql.Tx, id int64, status string) error {
	_, err := tx.Exec(`UPDATE tournament_matches SET status=$1 WHERE id=$2`, status, id)
	return err
}

func (r *Repo) SetMatchWinner(id int64, winnerTgID int64) error {
	_, err := r.db.Exec(`UPDATE tournament_matches SET winner_tg_id=$1, status='done' WHERE id=$2`, winnerTgID, id)
	return err
}

func (r *Repo) FillMatchSlot(id int64, slot int, playerTgID int64) error {
	if slot == 1 {
		_, err := r.db.Exec(`UPDATE tournament_matches SET player1_tg_id=$1 WHERE id=$2`, playerTgID, id)
		return err
	}
	_, err := r.db.Exec(`UPDATE tournament_matches SET player2_tg_id=$1 WHERE id=$2`, playerTgID, id)
	return err
}

func (r *Repo) GetBracket(tournamentID int64) ([]*models.Match, error) {
	rows, err := r.db.Query(`
		SELECT m.id, m.tournament_id, m.round, m.match_num,
		       m.player1_tg_id, m.player2_tg_id, m.winner_tg_id, m.status,
		       COALESCE(r1.user_name,''), COALESCE(r2.user_name,''), COALESCE(rw.user_name,'')
		FROM tournament_matches m
		LEFT JOIN tournament_registrations r1 ON r1.tournament_id=m.tournament_id AND r1.user_tg_id=m.player1_tg_id
		LEFT JOIN tournament_registrations r2 ON r2.tournament_id=m.tournament_id AND r2.user_tg_id=m.player2_tg_id
		LEFT JOIN tournament_registrations rw ON rw.tournament_id=m.tournament_id AND rw.user_tg_id=m.winner_tg_id
		WHERE m.tournament_id=$1
		ORDER BY m.round ASC, m.match_num ASC`, tournamentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*models.Match
	for rows.Next() {
		m := &models.Match{}
		if err := rows.Scan(&m.ID, &m.TournamentID, &m.Round, &m.MatchNum,
			&m.Player1TgID, &m.Player2TgID, &m.WinnerTgID, &m.Status,
			&m.Player1Name, &m.Player2Name, &m.WinnerName); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *Repo) GetMaxRound(tournamentID int64) (int, error) {
	var maxRound int
	err := r.db.QueryRow(`SELECT COALESCE(MAX(round),0) FROM tournament_matches WHERE tournament_id=$1`, tournamentID).Scan(&maxRound)
	return maxRound, err
}
