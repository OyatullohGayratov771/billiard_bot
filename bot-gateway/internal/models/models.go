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
