package bot

import "bot-gateway/internal/models"

// UserService — user-service bilan ishlash interfeysi
type UserService interface {
	RegisterOrGet(tgID int64, username, firstName, lastName string) (*models.User, error)
	GetByTelegramID(tgID int64) (*models.User, error)
	ListByRole(role string) ([]*models.User, error)
	ListStaff() ([]*models.User, error)
	SetRole(adminTgID, targetTgID int64, role string, branchID *int64) error
}

// TableService — table-service bilan ishlash interfeysi
type TableService interface {
	GetBranches() ([]*models.Branch, error)
	GetBranchByID(id int64) (*models.Branch, error)
	GetBranchTables(branchID int64) ([]*models.Table, error)
	GetTable(id int64) (*models.Table, error)
	StartSession(operator *models.User, tableID int64, clientName string) (*models.Session, error)
	EndSession(operator *models.User, tableID int64) (*models.Session, error)
	ActiveSession(tableID int64) (*models.Session, error)
	DailyReport(branchID int64) (int, int64, error)
	MonthlyReport(branchID int64) (int, int64, error)
	UpdateBranchNVR(id int64, ip string, port int, user, pass string) error
	SetTableRTSP(tableID int64, rtspURL string) error
}

// ClipService — clip-service bilan ishlash interfeysi
type ClipService interface {
	CreateRequest(in models.ClipRequestInput) (*models.ClipRequest, error)
	CreatePayment(clipID int64, screenshotFileID string) error
	ConfirmPayment(adminID, clipID int64) error
	SetStatus(adminID, clipID int64, status string) error
	GetByID(id int64) (*models.ClipRequest, error)
	ListByClient(tgID int64) ([]*models.ClipRequest, error)
	ListPending() ([]*models.ClipRequest, error)
	TriggerRecording(clipID int64) error
	DownloadClipFile(clipID int64) ([]byte, error)
}
