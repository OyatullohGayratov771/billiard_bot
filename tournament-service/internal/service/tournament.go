package service

import (
	cryptorand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"tournament-service/internal/models"
	"tournament-service/internal/repository"
)

type Service struct {
	repo *repository.Repo
	db   *sql.DB
}

func New(db *sql.DB, repo *repository.Repo) *Service {
	return &Service{repo: repo, db: db}
}

// ===================== TOURNAMENTS =====================

func (s *Service) CreateTournament(name string, branchID int64, tableID *int64, scheduledAt time.Time, price int64, maxPlayers int, adminTgID int64, joinCode string) (*models.Tournament, error) {
	if name == "" {
		return nil, errors.New("turnir nomi bo'sh bo'lmasin")
	}
	if maxPlayers < 2 {
		return nil, errors.New("max ishtirokchilar kamida 2 bo'lishi kerak")
	}
	if price < 0 {
		return nil, errors.New("narx manfiy bo'lishi mumkin emas")
	}
	if scheduledAt.Before(time.Now()) {
		return nil, errors.New("turnir sanasi o'tib ketgan")
	}
	return s.repo.CreateTournament(name, branchID, tableID, scheduledAt, price, maxPlayers, adminTgID, joinCode)
}

func (s *Service) GetTournament(id int64) (*models.Tournament, error) {
	return s.repo.GetTournament(id)
}

func (s *Service) ListTournaments(status string) ([]*models.Tournament, error) {
	return s.repo.ListTournaments(status)
}

func (s *Service) UpdateTournament(id int64, name string, scheduledAt time.Time, maxPlayers int) error {
	t, err := s.repo.GetTournament(id)
	if err != nil {
		return err
	}
	if t.Status != models.TournamentStatusRegistration {
		return errors.New("faqat ro'yxatga olish holatidagi turnirni tahrirlash mumkin")
	}
	if name == "" {
		return errors.New("turnir nomi bo'sh bo'lmasin")
	}
	if scheduledAt.Before(time.Now()) {
		return errors.New("turnir sanasi o'tib ketgan")
	}
	if maxPlayers < 2 {
		return errors.New("max ishtirokchilar kamida 2 bo'lishi kerak")
	}
	if maxPlayers < t.ApprovedCount {
		return fmt.Errorf("max o'yinchilar %d ta tasdiqlangan ishtirokchidan kam bo'lishi mumkin emas", t.ApprovedCount)
	}
	return s.repo.UpdateTournament(id, name, scheduledAt, maxPlayers)
}

func (s *Service) CancelTournament(id int64) error {
	t, err := s.repo.GetTournament(id)
	if err != nil {
		return err
	}
	if t.Status == models.TournamentStatusFinished {
		return errors.New("tugagan turnirni bekor qilib bo'lmaydi")
	}
	return s.repo.SetTournamentStatus(id, models.TournamentStatusCancelled)
}

// ===================== TV =====================

func (s *Service) GenerateTVToken(tournamentID int64) (string, error) {
	if _, err := s.repo.GetTournament(tournamentID); err != nil {
		return "", err
	}
	// Return existing token if already generated
	if tok, err := s.repo.GetTVTokenByTournament(tournamentID); err == nil {
		return tok, nil
	}
	buf := make([]byte, 16)
	if _, err := cryptorand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	if err := s.repo.SaveTVToken(token, tournamentID); err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) GetTVData(token string) (*models.TVData, error) {
	tv, err := s.repo.GetTVToken(token)
	if err != nil {
		return nil, err
	}
	t, err := s.repo.GetTournament(tv.TournamentID)
	if err != nil {
		return nil, err
	}
	matches, err := s.repo.GetBracket(tv.TournamentID)
	if err != nil {
		return nil, err
	}
	return &models.TVData{Tournament: t, Matches: matches}, nil
}

// ===================== REGISTRATIONS =====================

func (s *Service) RegisterManual(tournamentID int64, playerName string) (*models.Registration, error) {
	if playerName == "" {
		return nil, errors.New("o'yinchi nomi bo'sh bo'lmasin")
	}
	t, err := s.repo.GetTournament(tournamentID)
	if err != nil {
		return nil, errors.New("turnir topilmadi")
	}
	if t.Status != models.TournamentStatusRegistration {
		return nil, errors.New("turnirga ro'yxat yopilgan")
	}
	if t.ApprovedCount >= t.MaxPlayers {
		return nil, fmt.Errorf("turnir to'lgan (%d/%d)", t.ApprovedCount, t.MaxPlayers)
	}
	return s.repo.RegisterManual(tournamentID, playerName)
}

func (s *Service) Register(tournamentID, userTgID int64, userName, joinCode string) (*models.Registration, error) {
	t, err := s.repo.GetTournament(tournamentID)
	if err != nil {
		return nil, errors.New("turnir topilmadi")
	}
	if t.Status != models.TournamentStatusRegistration {
		return nil, errors.New("turnirga ro'yxat yopilgan")
	}
	if t.ApprovedCount >= t.MaxPlayers {
		return nil, fmt.Errorf("turnir to'lgan (%d/%d)", t.ApprovedCount, t.MaxPlayers)
	}
	if t.JoinCode != "" && joinCode != t.JoinCode {
		return nil, errors.New("noto'g'ri maxfiy kod")
	}
	return s.repo.Register(tournamentID, userTgID, userName)
}

func (s *Service) ListRegistrations(tournamentID int64) ([]*models.Registration, error) {
	return s.repo.ListRegistrations(tournamentID)
}

func (s *Service) ApproveRegistration(tournamentID, regID int64) error {
	reg, err := s.repo.GetRegistration(regID)
	if err != nil {
		return err
	}
	if reg.TournamentID != tournamentID {
		return errors.New("noto'g'ri turnir")
	}
	t, err := s.repo.GetTournament(tournamentID)
	if err != nil {
		return fmt.Errorf("turnir topilmadi: %v", err)
	}
	if t.ApprovedCount >= t.MaxPlayers {
		return fmt.Errorf("o'rin qolmadi (%d/%d to'lgan)", t.ApprovedCount, t.MaxPlayers)
	}
	return s.repo.SetRegistrationStatus(regID, models.RegStatusApproved)
}

func (s *Service) RejectRegistration(tournamentID, regID int64) error {
	reg, err := s.repo.GetRegistration(regID)
	if err != nil {
		return err
	}
	if reg.TournamentID != tournamentID {
		return errors.New("noto'g'ri turnir")
	}
	return s.repo.SetRegistrationStatus(regID, models.RegStatusRejected)
}

func (s *Service) GetUserRegistration(tournamentID, userTgID int64) (*models.Registration, error) {
	return s.repo.GetUserRegistration(tournamentID, userTgID)
}

func (s *Service) GetUserTournaments(userTgID int64) ([]*models.Registration, error) {
	return s.repo.GetUserTournaments(userTgID)
}

// ===================== BRACKET =====================

func nextPow2(n int) int {
	p := 1
	for p < n {
		p *= 2
	}
	return p
}

func totalRounds(size int) int {
	r := 0
	for size > 1 {
		size >>= 1
		r++
	}
	return r
}

func (s *Service) GenerateBracket(tournamentID int64) ([]*models.Match, error) {
	t, err := s.repo.GetTournament(tournamentID)
	if err != nil {
		return nil, errors.New("turnir topilmadi")
	}
	if t.Status != models.TournamentStatusRegistration {
		return nil, errors.New("bracket allaqachon yaratilgan yoki turnir tugagan")
	}

	regs, err := s.repo.ListApprovedRegistrations(tournamentID)
	if err != nil {
		return nil, err
	}
	N := len(regs)
	if N < 2 {
		return nil, errors.New("bracket uchun kamida 2 tasdiqlangan ishtirokchi kerak")
	}

	// Shuffle
	rand.Shuffle(N, func(i, j int) { regs[i], regs[j] = regs[j], regs[i] })

	size := nextPow2(N)
	rounds := totalRounds(size)

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Pre-create ALL matches as pending
	// matchID[round][matchNum] → db id
	type key struct{ r, m int }
	matchID := map[key]int64{}

	for r := 1; r <= rounds; r++ {
		count := size >> r // size / 2^r
		if count == 0 {
			count = 1
		}
		for m := 1; m <= count; m++ {
			id, err := s.repo.InsertMatch(tx, tournamentID, r, m)
			if err != nil {
				return nil, fmt.Errorf("match yaratishda xato (r=%d m=%d): %v", r, m, err)
			}
			matchID[key{r, m}] = id
		}
	}

	// Fill round-1 slots from shuffled players
	slots := make([]*int64, size)
	for i := range regs {
		tg := regs[i].UserTgID
		slots[i] = &tg
	}

	for i := 0; i < size/2; i++ {
		p1 := slots[2*i]
		p2 := slots[2*i+1]
		id := matchID[key{1, i + 1}]

		var status string
		switch {
		case p1 != nil && p2 != nil:
			status = models.MatchStatusReady
		case p1 != nil, p2 != nil:
			status = models.MatchStatusBye
		default:
			status = models.MatchStatusVoid
		}
		if err := s.repo.UpdateMatchPlayers(tx, id, p1, p2, status); err != nil {
			return nil, err
		}
	}

	// Cascade byes and voids upward round by round
	for r := 1; r < rounds; r++ {
		count := size >> r
		if count == 0 {
			count = 1
		}
		for m := 1; m <= count; m++ {
			id := matchID[key{r, m}]
			match, err := s.repo.GetMatchByRoundNum(tx, tournamentID, r, m)
			if err != nil {
				continue
			}

			nextR := r + 1
			nextM := (m + 1) / 2
			nextID := matchID[key{nextR, nextM}]
			slot := 1
			if m%2 == 0 {
				slot = 2
			}

			switch match.Status {
			case models.MatchStatusBye:
				// auto-advance the single player
				var winner int64
				if match.Player1TgID != nil {
					winner = *match.Player1TgID
				} else {
					winner = *match.Player2TgID
				}
				if _, err := tx.Exec(`UPDATE tournament_matches SET winner_tg_id=$1, status='done' WHERE id=$2`, winner, id); err != nil {
					return nil, fmt.Errorf("bye advance xatosi (r=%d m=%d): %v", r, m, err)
				}
				if err := s.fillSlotTx(tx, nextID, slot, winner); err != nil {
					return nil, fmt.Errorf("slot to'ldirishda xato (r=%d m=%d): %v", r, m, err)
				}

			case models.MatchStatusVoid:
				// nothing to advance — sibling match may cascade parent to bye/void
				_ = id
			}
		}

		// After processing, recompute any next-round matches that might now be bye/void
		nextCount := size >> (r + 1)
		if nextCount == 0 {
			nextCount = 1
		}
		for m := 1; m <= nextCount; m++ {
			nextID := matchID[key{r + 1, m}]
			c1ID := matchID[key{r, 2*m - 1}]
			c2ID := matchID[key{r, 2*m}]
			c1, _ := s.repo.GetMatchByRoundNum(tx, tournamentID, r, 2*m-1)
			c2, _ := s.repo.GetMatchByRoundNum(tx, tournamentID, r, 2*m)
			nm, _ := s.repo.GetMatchByRoundNum(tx, tournamentID, r+1, m)
			if c1 == nil || c2 == nil || nm == nil {
				continue
			}
			_ = c1ID
			_ = c2ID

			switch {
			case c1.Status == models.MatchStatusVoid && c2.Status == models.MatchStatusVoid:
				s.repo.UpdateMatchStatus(tx, nextID, models.MatchStatusVoid)
			case c1.Status == models.MatchStatusVoid && nm.Player2TgID != nil:
				s.repo.UpdateMatchStatus(tx, nextID, models.MatchStatusBye)
			case c2.Status == models.MatchStatusVoid && nm.Player1TgID != nil:
				s.repo.UpdateMatchStatus(tx, nextID, models.MatchStatusBye)
			case nm.Player1TgID != nil && nm.Player2TgID != nil:
				s.repo.UpdateMatchStatus(tx, nextID, models.MatchStatusReady)
			}
		}
	}

	if err := s.repo.SetTournamentStatus(tournamentID, models.TournamentStatusInProgress); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("bracket saqlashda xato: %v", err)
	}
	return s.repo.GetBracket(tournamentID)
}

func (s *Service) fillSlotTx(tx *sql.Tx, matchID int64, slot int, playerTgID int64) error {
	var err error
	if slot == 1 {
		_, err = tx.Exec(`UPDATE tournament_matches SET player1_tg_id=$1 WHERE id=$2`, playerTgID, matchID)
	} else {
		_, err = tx.Exec(`UPDATE tournament_matches SET player2_tg_id=$1 WHERE id=$2`, playerTgID, matchID)
	}
	return err
}

func (s *Service) GetBracket(tournamentID int64) ([]*models.Match, error) {
	return s.repo.GetBracket(tournamentID)
}

// ===================== MATCH RESULT =====================

// SetResult records winner, advances to next round. Returns nextMatchID (0 if final).
func (s *Service) SetResult(matchID, winnerTgID int64) (nextMatchID int64, finished bool, err error) {
	m, err := s.repo.GetMatch(matchID)
	if err != nil {
		return 0, false, errors.New("o'yin topilmadi")
	}
	if m.Status != models.MatchStatusReady {
		return 0, false, fmt.Errorf("o'yin hali tayyor emas (holat: %s)", m.Status)
	}
	isP1 := m.Player1TgID != nil && *m.Player1TgID == winnerTgID
	isP2 := m.Player2TgID != nil && *m.Player2TgID == winnerTgID
	if !isP1 && !isP2 {
		return 0, false, errors.New("bu o'yinchi ushbu o'yinda yo'q")
	}

	if err := s.repo.SetMatchWinner(matchID, winnerTgID); err != nil {
		return 0, false, err
	}

	maxRound, _ := s.repo.GetMaxRound(m.TournamentID)
	nextRound := m.Round + 1

	if nextRound > maxRound {
		_ = s.repo.SetTournamentStatus(m.TournamentID, models.TournamentStatusFinished)
		return 0, true, nil
	}

	nextMatchNum := (m.MatchNum + 1) / 2
	slot := 1
	if m.MatchNum%2 == 0 {
		slot = 2
	}

	// Get next match ID
	var nmID int64
	if err := s.db.QueryRow(
		`SELECT id FROM tournament_matches WHERE tournament_id=$1 AND round=$2 AND match_num=$3`,
		m.TournamentID, nextRound, nextMatchNum,
	).Scan(&nmID); err != nil {
		return 0, false, fmt.Errorf("keyingi o'yin topilmadi: %v", err)
	}

	if err := s.repo.FillMatchSlot(nmID, slot, winnerTgID); err != nil {
		return 0, false, err
	}

	// Check if next match is now ready
	var p1, p2 *int64
	if err := s.db.QueryRow(`SELECT player1_tg_id, player2_tg_id FROM tournament_matches WHERE id=$1`, nmID).Scan(&p1, &p2); err != nil {
		return 0, false, fmt.Errorf("keyingi o'yin tekshirishda xato: %v", err)
	}
	if p1 != nil && p2 != nil {
		if _, err := s.db.Exec(`UPDATE tournament_matches SET status='ready' WHERE id=$1`, nmID); err != nil {
			return 0, false, fmt.Errorf("keyingi o'yin holatini yangilashda xato: %v", err)
		}
	}

	return nmID, false, nil
}
