package models

import "time"

const (
	TournamentStatusRegistration = "registration"
	TournamentStatusInProgress   = "in_progress"
	TournamentStatusFinished     = "finished"
	TournamentStatusCancelled    = "cancelled"
)

const (
	TournamentTypeSingleElim = "single_elimination"
	TournamentTypeDoubleElim = "double_elimination"
	TournamentTypeRoundRobin = "round_robin"
)

const (
	RegStatusPending  = "pending"
	RegStatusApproved = "approved"
	RegStatusRejected = "rejected"
)

const (
	MatchStatusPending    = "pending"     // players not assigned yet
	MatchStatusReady      = "ready"       // both players assigned, awaiting start
	MatchStatusInProgress = "in_progress" // match started, admin entered table num
	MatchStatusBye        = "bye"         // one player auto-advances
	MatchStatusVoid       = "void"        // no players (double-bye cascade)
	MatchStatusDone       = "done"        // winner recorded
)

const (
	MatchTypeWinners    = "winners"
	MatchTypeLosers     = "losers"
	MatchTypeGrandFinal = "grand_final"
	MatchTypeRoundRobin = "round_robin"
)

// LBRoundOffset — losers bracket rounds are stored as lbRoundOffset+lr to avoid UNIQUE constraint conflicts.
const LBRoundOffset = 1000
const GFRound = 2000

type Tournament struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	BranchID    int64     `json:"branch_id"`
	TableID     *int64    `json:"table_id"`
	ScheduledAt time.Time `json:"scheduled_at"`
	Price       int64     `json:"price"`
	MaxPlayers  int       `json:"max_players"`
	Status      string    `json:"status"`
	Type        string    `json:"type"`
	CreatedBy   int64     `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`

	JoinCode   string `json:"join_code"`
	TableCount int    `json:"table_count"`

	BranchName string `json:"branch_name"`
	TableNum   int    `json:"table_num"`

	ApprovedCount int `json:"approved_count"`
}

type Registration struct {
	ID             int64      `json:"id"`
	TournamentID   int64      `json:"tournament_id"`
	TournamentName string     `json:"tournament_name"`
	UserTgID       int64      `json:"user_tg_id"`
	UserName       string     `json:"user_name"`
	Status         string     `json:"status"`
	RegisteredAt   time.Time  `json:"registered_at"`
	DecidedAt      *time.Time `json:"decided_at"`
}

type TVToken struct {
	Token        string    `json:"token"`
	TournamentID int64     `json:"tournament_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type TVData struct {
	Tournament *Tournament `json:"tournament"`
	Matches    []*Match    `json:"matches"`
}

type Match struct {
	ID           int64  `json:"id"`
	TournamentID int64  `json:"tournament_id"`
	Round        int    `json:"round"`
	MatchNum     int    `json:"match_num"`
	Player1TgID  *int64 `json:"player1_tg_id"`
	Player2TgID  *int64 `json:"player2_tg_id"`
	WinnerTgID   *int64 `json:"winner_tg_id"`
	Status       string `json:"status"`
	MatchType    string `json:"match_type"`
	TableNum     int    `json:"table_num"`
	Player1Score int    `json:"player1_score"`
	Player2Score int    `json:"player2_score"`

	LoserNextMatchID  *int64 `json:"-"`
	WinnerNextMatchID *int64 `json:"-"`

	Player1Name string `json:"player1_name"`
	Player2Name string `json:"player2_name"`
	WinnerName  string `json:"winner_name"`
	WinnerSlot  int    `json:"winner_slot"` // 1 or 2, 0 if no winner
}
