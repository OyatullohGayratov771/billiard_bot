package models

import "time"

// --- Rollar ---

const (
	RoleSuperadmin = "superadmin"
	RoleAdmin      = "admin"
	RoleOperator   = "operator"
	RoleClient     = "client"
)

// --- Stol holatlari ---

const (
	TableStatusFree = "free"
	TableStatusBusy = "busy"
)

// --- Klip so'rov holatlari ---

const (
	ClipStatusPending    = "pending"
	ClipStatusPaid       = "paid"
	ClipStatusProcessing = "processing"
	ClipStatusDone       = "done"
	ClipStatusFailed     = "failed"
	ClipStatusRefunded   = "refunded"
)

// --- To'lov metodlari ---

const (
	PaymentMethodManual = "manual"
	PaymentMethodClick  = "click"
	PaymentMethodPayme  = "payme"
)

// --- Narxlar ---

const (
	ClipPrice     = int64(1_000_000) // 10,000 so'm = 1,000,000 tiyin
	ClipPriceSom  = 10_000
)

// ===================== MODELLAR =====================

type Branch struct {
	ID        int64
	Name      string
	Address   string
	NVRHost   string
	NVRPort   int
	NVRUser   string
	NVRPass   string
	CreatedAt time.Time
}

type Table struct {
	ID            int64
	BranchID      int64
	TableNum      int
	CameraChannel int
	RTSPUrl       string
	Status        string
	PricePerHour  int64 // tiyin
	CreatedAt     time.Time

	// join fields
	BranchName string
}

type User struct {
	ID         int64
	TelegramID int64
	Username   string
	FirstName  string
	LastName   string
	Phone      string
	Role       string
	BranchID   *int64
	IsActive   bool
	CreatedAt  time.Time
}

func (u *User) DisplayName() string {
	if u.FirstName != "" {
		return u.FirstName
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return "Foydalanuvchi"
}

func (u *User) IsSuperadmin() bool { return u.Role == RoleSuperadmin }
func (u *User) IsAdmin() bool      { return u.Role == RoleAdmin }
func (u *User) IsOperator() bool   { return u.Role == RoleOperator }
func (u *User) IsStaff() bool {
	return u.Role == RoleSuperadmin || u.Role == RoleAdmin || u.Role == RoleOperator
}

type ClipRequest struct {
	ID         int64
	ClientTgID int64
	ClientName string
	BranchID   int64
	TableID    int64
	StartTime  time.Time
	EndTime    time.Time
	Status     string
	ClipPath   string
	Notes      string
	CreatedAt  time.Time

	// join fields
	BranchName string
	TableNum   int
}

type Payment struct {
	ID             int64
	ClipRequestID  int64
	Amount         int64
	Method         string
	Status         string
	ProviderID     string
	ScreenshotID   string
	PaidAt         *time.Time
	CreatedAt      time.Time
}

type AuditLog struct {
	ID        int64
	UserID    *int64
	Action    string
	Details   string
	CreatedAt time.Time
}

// ===================== TURNIR MODELLARI =====================

const (
	TournamentStatusRegistration = "registration"
	TournamentStatusInProgress   = "in_progress"
	TournamentStatusFinished     = "finished"
	TournamentStatusCancelled    = "cancelled"
)

const (
	RegStatusPending  = "pending"
	RegStatusApproved = "approved"
	RegStatusRejected = "rejected"
)

const (
	MatchStatusPending = "pending"
	MatchStatusReady   = "ready"
	MatchStatusBye     = "bye"
	MatchStatusVoid    = "void"
	MatchStatusDone    = "done"
)

type Tournament struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	BranchID      int64     `json:"branch_id"`
	TableID       *int64    `json:"table_id"`
	ScheduledAt   time.Time `json:"scheduled_at"`
	Price         int64     `json:"price"`
	MaxPlayers    int       `json:"max_players"`
	Status        string    `json:"status"`
	CreatedBy     int64     `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	JoinCode      string    `json:"join_code"`
	BranchName    string    `json:"branch_name"`
	TableNum      int       `json:"table_num"`
	ApprovedCount int       `json:"approved_count"`
}

type TournamentRegistration struct {
	ID             int64      `json:"id"`
	TournamentID   int64      `json:"tournament_id"`
	TournamentName string     `json:"tournament_name"`
	UserTgID       int64      `json:"user_tg_id"`
	UserName       string     `json:"user_name"`
	Status         string     `json:"status"`
	RegisteredAt   time.Time  `json:"registered_at"`
	DecidedAt      *time.Time `json:"decided_at"`
}

type TournamentMatch struct {
	ID           int64  `json:"id"`
	TournamentID int64  `json:"tournament_id"`
	Round        int    `json:"round"`
	MatchNum     int    `json:"match_num"`
	Player1TgID  *int64 `json:"player1_tg_id"`
	Player2TgID  *int64 `json:"player2_tg_id"`
	WinnerTgID   *int64 `json:"winner_tg_id"`
	Status       string `json:"status"`
	Player1Name  string `json:"player1_name"`
	Player2Name  string `json:"player2_name"`
	WinnerName   string `json:"winner_name"`
}

// ===================== DTO lar =====================

type ClipRequestInput struct {
	ClientTgID int64
	ClientName string
	BranchID   int64
	TableID    int64
	StartTime  time.Time
	EndTime    time.Time
}

// PricePerHourSom — stolning soatlik narxini so'mda qaytaradi
func PricePerHourSom(pricePerHour int64) int64 {
	return pricePerHour / 100
}
