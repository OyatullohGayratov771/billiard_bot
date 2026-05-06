package bot

import (
	"time"

	"bot-gateway/internal/models"
)

// UserService — user-service bilan ishlash interfeysi
type UserService interface {
	RegisterOrGet(tgID int64, username, firstName, lastName string) (*models.User, error)
	GetByTelegramID(tgID int64) (*models.User, error)
	ListByRole(role string) ([]*models.User, error)
	ListStaff() ([]*models.User, error)
	SetRole(adminTgID, targetTgID int64, role string, branchID *int64) error
	SavePhone(tgID int64, phone string) error
	UpdateName(tgID int64, firstName string) error
}

// TableService — table-service bilan ishlash interfeysi
type TableService interface {
	GetBranches() ([]*models.Branch, error)
	GetBranchByID(id int64) (*models.Branch, error)
	GetBranchTables(branchID int64) ([]*models.Table, error)
	GetTable(id int64) (*models.Table, error)
	UpdateBranchNVR(id int64, ip string, port int, user, pass string) error
	SetTableRTSP(tableID int64, rtspURL string) error
}

// TournamentService — tournament-service bilan ishlash interfeysi
type TournamentService interface {
	CreateTournament(name string, branchID int64, tableID *int64, scheduledAt time.Time, price int64, maxPlayers int, adminTgID int64, joinCode string, tournamentType string) (*models.Tournament, error)
	ListTournaments(status string) ([]*models.Tournament, error)
	GetTournament(id int64) (*models.Tournament, error)
	CancelTournament(id int64) error
	UpdateTournament(id int64, name string, scheduledAt time.Time, maxPlayers int) error
	Register(tournamentID, userTgID int64, userName, joinCode string) (*models.TournamentRegistration, error)
	RegisterManual(tournamentID int64, playerName string) (*models.TournamentRegistration, error)
	ListRegistrations(tournamentID int64) ([]*models.TournamentRegistration, error)
	ApproveRegistration(tournamentID, regID int64) error
	RejectRegistration(tournamentID, regID int64) error
	GenerateBracket(tournamentID int64) ([]*models.TournamentMatch, error)
	GetBracket(tournamentID int64) ([]*models.TournamentMatch, error)
	SetResult(matchID, winnerTgID int64) (winnerNextID int64, loserNextID int64, finished bool, err error)
	GetUserTournaments(tgID int64) ([]*models.TournamentRegistration, error)
	GetUserRegistration(tournamentID, userTgID int64) (*models.TournamentRegistration, error)
	GenerateTVToken(tournamentID int64) (string, error)
	GetTVTokenByTournament(tournamentID int64) (string, error)
	PushTVUpdate(token string) (int, error)
}

// ClipService — clip-service bilan ishlash interfeysi
type ClipService interface {
	CreateRequest(in models.ClipRequestInput) (*models.ClipRequest, error)
	CreatePayment(clipID int64, screenshotFileID string) error
	ConfirmPayment(adminID, clipID int64) error
	SetStatus(adminID, clipID int64, status, note string) error
	GetByID(id int64) (*models.ClipRequest, error)
	ListByClient(tgID int64) ([]*models.ClipRequest, error)
	ListPending() ([]*models.ClipRequest, error)
	ListRecent(limit int) ([]*models.ClipRequest, error)
	TriggerRecording(clipID int64) error
	DownloadClipFile(clipID int64) ([]byte, error)
}
