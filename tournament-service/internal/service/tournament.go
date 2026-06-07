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

func (s *Service) CreateTournament(name string, branchID int64, tableID *int64, scheduledAt time.Time, price int64, maxPlayers int, adminTgID int64, joinCode string, tournamentType string, tableCount int, slotMinutes int) (*models.Tournament, error) {
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
	switch tournamentType {
	case models.TournamentTypeSingleElim, models.TournamentTypeDoubleElim, models.TournamentTypeRoundRobin:
	case "":
		tournamentType = models.TournamentTypeSingleElim
	default:
		return nil, fmt.Errorf("noto'g'ri turnir turi: %s", tournamentType)
	}
	if slotMinutes <= 0 {
		slotMinutes = 30
	}
	return s.repo.CreateTournament(name, branchID, tableID, scheduledAt, price, maxPlayers, adminTgID, joinCode, tournamentType, tableCount, slotMinutes)
}

func (s *Service) SetMatchScheduledAt(matchID int64, scheduledAt time.Time) error {
	return s.repo.SetMatchScheduledAt(matchID, scheduledAt)
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

func (s *Service) DeleteTournament(id int64) error {
	t, err := s.repo.GetTournament(id)
	if err != nil {
		return err
	}
	if t.Status == models.TournamentStatusInProgress {
		return errors.New("davom etayotgan turnirni o'chirib bo'lmaydi")
	}
	return s.repo.DeleteTournament(id)
}

// ===================== TV =====================

func (s *Service) GenerateTVToken(tournamentID int64) (string, error) {
	if _, err := s.repo.GetTournament(tournamentID); err != nil {
		return "", err
	}
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
		return nil, errors.New("faqat ro'yxatga olish holatidagi turnir uchun bracket yaratish mumkin")
	}
	if err := s.repo.DeleteBracket(tournamentID); err != nil {
		return nil, fmt.Errorf("eski bracket o'chirishda xato: %v", err)
	}
	var matches []*models.Match
	switch t.Type {
	case models.TournamentTypeDoubleElim:
		matches, err = s.generateDoubleElimBracket(tournamentID)
	case models.TournamentTypeRoundRobin:
		matches, err = s.generateRoundRobinBracket(tournamentID)
	default:
		matches, err = s.generateSingleElimBracket(tournamentID)
	}
	if err != nil {
		return nil, err
	}
	// Auto-assign tables and match times to ready matches if table_count is set
	if t.TableCount > 0 {
		tableIdx := 1
		for _, m := range matches {
			if m.Status == models.MatchStatusReady && tableIdx <= t.TableCount {
				_ = s.repo.SetMatchTableNum(m.ID, tableIdx)
				m.TableNum = tableIdx
				// Slot 0 for all first-round matches → start at tournament time
				if !t.ScheduledAt.IsZero() {
					slotIdx := s.repo.CountPrevMatchesAtTable(t.ID, tableIdx, m.ID)
					slotTime := t.ScheduledAt.Add(time.Duration(slotIdx*t.SlotMinutes) * time.Minute)
					_ = s.repo.SetMatchScheduledAt(m.ID, slotTime)
					m.MatchScheduledAt = &slotTime
				}
				tableIdx++
			}
		}
	}
	return matches, nil
}

func (s *Service) ShuffleBracket(tournamentID int64) ([]*models.Match, error) {
	return s.GenerateBracket(tournamentID)
}

func (s *Service) SetMatchTable(matchID int64, tableNum int) error {
	m, err := s.repo.GetMatch(matchID)
	if err != nil {
		return errors.New("o'yin topilmadi")
	}
	if m.Status != models.MatchStatusReady && m.Status != models.MatchStatusInProgress {
		return fmt.Errorf("bu o'yinga stol belgilab bo'lmaydi (holat: %s)", m.Status)
	}
	if err := s.repo.SetMatchTableNum(matchID, tableNum); err != nil {
		return err
	}
	// Auto-calculate scheduled time for this match slot
	t, err2 := s.repo.GetTournament(m.TournamentID)
	if err2 == nil && !t.ScheduledAt.IsZero() && t.SlotMinutes > 0 {
		slotIdx := s.repo.CountPrevMatchesAtTable(t.ID, tableNum, matchID)
		slotTime := t.ScheduledAt.Add(time.Duration(slotIdx*t.SlotMinutes) * time.Minute)
		_ = s.repo.SetMatchScheduledAt(matchID, slotTime)
	}
	return nil
}

func (s *Service) StartMatch(matchID int64, tableNum int) error {
	m, err := s.repo.GetMatch(matchID)
	if err != nil {
		return errors.New("o'yin topilmadi")
	}
	if m.Status != models.MatchStatusReady {
		return fmt.Errorf("o'yin boshlash uchun tayyor emas (holat: %s)", m.Status)
	}
	effective := tableNum
	if effective == 0 {
		effective = m.TableNum
	}
	if err := s.repo.StartMatch(matchID, effective); err != nil {
		return err
	}
	t, err := s.repo.GetTournament(m.TournamentID)
	if err != nil {
		return nil
	}
	if t.Status == models.TournamentStatusRegistration {
		_ = s.repo.SetTournamentStatus(m.TournamentID, models.TournamentStatusInProgress)
	}
	return nil
}

// --- Single Elimination ---

func (s *Service) generateSingleElimBracket(tournamentID int64) ([]*models.Match, error) {
	regs, err := s.repo.ListApprovedRegistrations(tournamentID)
	if err != nil {
		return nil, err
	}
	N := len(regs)
	if N < 2 {
		return nil, errors.New("bracket uchun kamida 2 tasdiqlangan ishtirokchi kerak")
	}

	rand.Shuffle(N, func(i, j int) { regs[i], regs[j] = regs[j], regs[i] })

	size := nextPow2(N)
	rounds := totalRounds(size)

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	type key struct{ r, m int }
	matchID := map[key]int64{}

	for r := 1; r <= rounds; r++ {
		count := size >> r
		if count == 0 {
			count = 1
		}
		for m := 1; m <= count; m++ {
			id, err := s.repo.InsertMatch(tx, tournamentID, r, m, models.MatchTypeWinners)
			if err != nil {
				return nil, fmt.Errorf("match yaratishda xato (r=%d m=%d): %v", r, m, err)
			}
			matchID[key{r, m}] = id
		}
	}

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

	// Cascade byes and voids upward
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
				_ = id
			}
		}

		nextCount := size >> (r + 1)
		if nextCount == 0 {
			nextCount = 1
		}
		for m := 1; m <= nextCount; m++ {
			nextID := matchID[key{r + 1, m}]
			c1, _ := s.repo.GetMatchByRoundNum(tx, tournamentID, r, 2*m-1)
			c2, _ := s.repo.GetMatchByRoundNum(tx, tournamentID, r, 2*m)
			nm, _ := s.repo.GetMatchByRoundNum(tx, tournamentID, r+1, m)
			if c1 == nil || c2 == nil || nm == nil {
				continue
			}

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

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("bracket saqlashda xato: %v", err)
	}
	return s.repo.GetBracket(tournamentID)
}

// --- Round Robin ---

func (s *Service) generateRoundRobinBracket(tournamentID int64) ([]*models.Match, error) {
	regs, err := s.repo.ListApprovedRegistrations(tournamentID)
	if err != nil {
		return nil, err
	}
	N := len(regs)
	if N < 2 {
		return nil, errors.New("Round Robin uchun kamida 2 tasdiqlangan ishtirokchi kerak")
	}

	rand.Shuffle(N, func(i, j int) { regs[i], regs[j] = regs[j], regs[i] })

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	matchNum := 1
	for i := 0; i < N-1; i++ {
		for j := i + 1; j < N; j++ {
			id, err := s.repo.InsertMatch(tx, tournamentID, 1, matchNum, models.MatchTypeRoundRobin)
			if err != nil {
				return nil, fmt.Errorf("RR match yaratishda xato (%d vs %d): %v", i, j, err)
			}
			p1 := regs[i].UserTgID
			p2 := regs[j].UserTgID
			if err := s.repo.UpdateMatchPlayers(tx, id, &p1, &p2, models.MatchStatusReady); err != nil {
				return nil, err
			}
			matchNum++
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("RR bracket saqlashda xato: %v", err)
	}
	return s.repo.GetBracket(tournamentID)
}

// --- Double Elimination ---

func (s *Service) generateDoubleElimBracket(tournamentID int64) ([]*models.Match, error) {
	regs, err := s.repo.ListApprovedRegistrations(tournamentID)
	if err != nil {
		return nil, err
	}
	N := len(regs)
	size := nextPow2(N)
	if size != N || N < 4 {
		return nil, fmt.Errorf("Double Elimination uchun ishtirokchilar soni 4, 8 yoki 16 bo'lishi kerak (hozir: %d)", N)
	}
	k := totalRounds(size) // log2(size)

	rand.Shuffle(N, func(i, j int) { regs[i], regs[j] = regs[j], regs[i] })

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	type key struct{ r, m int }
	ids := map[key]int64{}

	// Create WB matches (rounds 1..k)
	for r := 1; r <= k; r++ {
		count := size >> r
		if count == 0 {
			count = 1
		}
		for m := 1; m <= count; m++ {
			id, err := s.repo.InsertMatch(tx, tournamentID, r, m, models.MatchTypeWinners)
			if err != nil {
				return nil, fmt.Errorf("WB match yaratishda xato (r=%d m=%d): %v", r, m, err)
			}
			ids[key{r, m}] = id
		}
	}

	// Create LB matches (lbRoundOffset+1 .. lbRoundOffset+2*(k-1))
	lbRounds := 2 * (k - 1)
	for lr := 1; lr <= lbRounds; lr++ {
		count := size >> ((lr + 3) / 2)
		if count == 0 {
			count = 1
		}
		for m := 1; m <= count; m++ {
			id, err := s.repo.InsertMatch(tx, tournamentID, models.LBRoundOffset+lr, m, models.MatchTypeLosers)
			if err != nil {
				return nil, fmt.Errorf("LB match yaratishda xato (lr=%d m=%d): %v", lr, m, err)
			}
			ids[key{models.LBRoundOffset + lr, m}] = id
		}
	}

	// Create Grand Final
	gfID, err := s.repo.InsertMatch(tx, tournamentID, models.GFRound, 1, models.MatchTypeGrandFinal)
	if err != nil {
		return nil, fmt.Errorf("Grand Final yaratishda xato: %v", err)
	}
	ids[key{models.GFRound, 1}] = gfID

	// Fill WB R1 with all N players (no byes since N = power of 2)
	for i := 0; i < size/2; i++ {
		p1 := regs[2*i].UserTgID
		p2 := regs[2*i+1].UserTgID
		if err := s.repo.UpdateMatchPlayers(tx, ids[key{1, i + 1}], &p1, &p2, models.MatchStatusReady); err != nil {
			return nil, err
		}
	}

	// WB winner routing: WB Rr Mm → WB R(r+1) M⌈m/2⌉
	for r := 1; r < k; r++ {
		count := size >> r
		if count == 0 {
			count = 1
		}
		for m := 1; m <= count; m++ {
			nextM := (m + 1) / 2
			if err := s.repo.SetWinnerNextMatch(tx, ids[key{r, m}], ids[key{r + 1, nextM}]); err != nil {
				return nil, err
			}
		}
	}
	// WB final → GF
	if err := s.repo.SetWinnerNextMatch(tx, ids[key{k, 1}], gfID); err != nil {
		return nil, err
	}

	// WB R1 loser routing → LB R1: W1Mm loser → L1M⌈m/2⌉
	wbR1Count := size / 2
	for m := 1; m <= wbR1Count; m++ {
		lbM := (m + 1) / 2
		if err := s.repo.SetLoserNextMatch(tx, ids[key{1, m}], ids[key{models.LBRoundOffset + 1, lbM}]); err != nil {
			return nil, err
		}
	}
	// WB Rr (r=2..k) loser routing → LB R(2r-2) Mm
	for r := 2; r <= k; r++ {
		count := size >> r
		if count == 0 {
			count = 1
		}
		lbR := 2 * (r - 1)
		for m := 1; m <= count; m++ {
			if err := s.repo.SetLoserNextMatch(tx, ids[key{r, m}], ids[key{models.LBRoundOffset + lbR, m}]); err != nil {
				return nil, err
			}
		}
	}

	// LB winner routing
	for lr := 1; lr <= lbRounds; lr++ {
		count := size >> ((lr + 3) / 2)
		if count == 0 {
			count = 1
		}
		if lr == lbRounds {
			// LB final → GF
			if err := s.repo.SetWinnerNextMatch(tx, ids[key{models.LBRoundOffset + lr, 1}], gfID); err != nil {
				return nil, err
			}
		} else {
			nextLR := lr + 1
			for m := 1; m <= count; m++ {
				var nextM int
				if nextLR%2 == 0 {
					// Even LB (drop round): same match number
					nextM = m
				} else {
					// Odd LB (consolidation): pair up
					nextM = (m + 1) / 2
				}
				if err := s.repo.SetWinnerNextMatch(tx, ids[key{models.LBRoundOffset + lr, m}], ids[key{models.LBRoundOffset + nextLR, nextM}]); err != nil {
					return nil, err
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("DE bracket saqlashda xato: %v", err)
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

func (s *Service) GetMatch(matchID int64) (*models.Match, error) {
	return s.repo.GetMatch(matchID)
}

// ===================== MATCH RESULT =====================

// SetResult records winner, advances players.
// Returns (winnerNextID, loserNextID, finished, trnID, assignedMatchID, assignedTableNum, err).
// assignedMatchID > 0 means a newly ready match was auto-assigned a freed table.
func (s *Service) SetResult(matchID, winnerTgID int64) (winnerNextID, loserNextID, assignedMatchID int64, assignedTableNum int, finished bool, trnID int64, err error) {
	m, err := s.repo.GetMatch(matchID)
	if err != nil {
		return 0, 0, 0, 0, false, 0, errors.New("o'yin topilmadi")
	}
	if m.Status != models.MatchStatusInProgress {
		return 0, 0, 0, 0, false, 0, fmt.Errorf("o'yin boshlanmagan (holat: %s)", m.Status)
	}
	isP1 := m.Player1TgID != nil && *m.Player1TgID == winnerTgID
	isP2 := m.Player2TgID != nil && *m.Player2TgID == winnerTgID
	if !isP1 && !isP2 {
		return 0, 0, 0, 0, false, 0, errors.New("bu o'yinchi ushbu o'yinda yo'q")
	}

	if err := s.repo.SetMatchWinner(matchID, winnerTgID); err != nil {
		return 0, 0, 0, 0, false, 0, err
	}

	t, err := s.repo.GetTournament(m.TournamentID)
	if err != nil {
		return 0, 0, 0, 0, false, 0, err
	}

	var wn, ln int64
	var fin bool
	var advErr error
	switch t.Type {
	case models.TournamentTypeRoundRobin:
		allDone, _ := s.repo.AllMatchesDone(m.TournamentID)
		if allDone {
			_ = s.repo.SetTournamentStatus(m.TournamentID, models.TournamentStatusFinished)
			fin = true
		}
	case models.TournamentTypeDoubleElim:
		wn, ln, fin, advErr = s.advanceDEResult(m, winnerTgID)
	default:
		wn, fin, advErr = s.advanceSEResult(m, winnerTgID)
	}
	if advErr != nil {
		return 0, 0, 0, 0, false, m.TournamentID, advErr
	}

	// Auto-reassign freed table to next waiting match
	if !fin && m.TableNum > 0 && t.TableCount > 0 {
		if next, _ := s.repo.GetNextReadyMatchWithoutTable(m.TournamentID); next != nil {
			_ = s.repo.SetMatchTableNum(next.ID, m.TableNum)
			assignedMatchID = next.ID
			assignedTableNum = m.TableNum
			// Auto-calc scheduled time: slot = number of previous matches at this table
			if !t.ScheduledAt.IsZero() && t.SlotMinutes > 0 {
				slotIdx := s.repo.CountPrevMatchesAtTable(t.ID, m.TableNum, next.ID)
				slotTime := t.ScheduledAt.Add(time.Duration(slotIdx*t.SlotMinutes) * time.Minute)
				_ = s.repo.SetMatchScheduledAt(next.ID, slotTime)
			}
		}
	}

	return wn, ln, assignedMatchID, assignedTableNum, fin, m.TournamentID, nil
}

func (s *Service) advanceSEResult(m *models.Match, winnerTgID int64) (nextMatchID int64, finished bool, err error) {
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

	var p1, p2 *int64
	_ = s.db.QueryRow(`SELECT player1_tg_id, player2_tg_id FROM tournament_matches WHERE id=$1`, nmID).Scan(&p1, &p2)
	if p1 != nil && p2 != nil {
		_, _ = s.db.Exec(`UPDATE tournament_matches SET status='ready' WHERE id=$1`, nmID)
	}

	return nmID, false, nil
}

func (s *Service) advanceDEResult(m *models.Match, winnerTgID int64) (winnerNextID int64, loserNextID int64, finished bool, err error) {
	if m.MatchType == models.MatchTypeGrandFinal {
		_ = s.repo.SetTournamentStatus(m.TournamentID, models.TournamentStatusFinished)
		return 0, 0, true, nil
	}

	// Determine loser
	var loserTgID int64
	if m.Player1TgID != nil && m.Player2TgID != nil {
		if *m.Player1TgID == winnerTgID {
			loserTgID = *m.Player2TgID
		} else {
			loserTgID = *m.Player1TgID
		}
	}

	// Advance winner via winner_next_match_id
	if m.WinnerNextMatchID != nil {
		nmID := *m.WinnerNextMatchID
		if err := s.fillAvailableSlot(nmID, winnerTgID); err != nil {
			return 0, 0, false, err
		}
		s.checkAndSetReady(nmID)
		winnerNextID = nmID
	}

	// Advance loser to LB (only for WB matches)
	if m.MatchType == models.MatchTypeWinners && m.LoserNextMatchID != nil && loserTgID != 0 {
		lbID := *m.LoserNextMatchID
		if err := s.fillAvailableSlot(lbID, loserTgID); err != nil {
			return winnerNextID, 0, false, fmt.Errorf("LB slot fill error: %v", err)
		}
		s.checkAndSetReady(lbID)
		loserNextID = lbID
	}

	return winnerNextID, loserNextID, false, nil
}

func (s *Service) fillAvailableSlot(matchID int64, playerTgID int64) error {
	var p1, p2 *int64
	if err := s.db.QueryRow(`SELECT player1_tg_id, player2_tg_id FROM tournament_matches WHERE id=$1`, matchID).Scan(&p1, &p2); err != nil {
		return err
	}
	if p1 == nil {
		_, err := s.db.Exec(`UPDATE tournament_matches SET player1_tg_id=$1 WHERE id=$2`, playerTgID, matchID)
		return err
	}
	if p2 == nil {
		_, err := s.db.Exec(`UPDATE tournament_matches SET player2_tg_id=$1 WHERE id=$2`, playerTgID, matchID)
		return err
	}
	return fmt.Errorf("match %d ikki o'rni ham to'lgan", matchID)
}

func (s *Service) checkAndSetReady(matchID int64) {
	var p1, p2 *int64
	var status string
	if err := s.db.QueryRow(`SELECT player1_tg_id, player2_tg_id, status FROM tournament_matches WHERE id=$1`, matchID).Scan(&p1, &p2, &status); err != nil {
		return
	}
	if status == models.MatchStatusPending && p1 != nil && p2 != nil {
		_, _ = s.db.Exec(`UPDATE tournament_matches SET status='ready' WHERE id=$1`, matchID)
	}
}
