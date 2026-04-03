package models

import "time"

const (
	RoleSuperadmin = "superadmin"
	RoleAdmin      = "admin"
	RoleOperator   = "operator"
	RoleClient     = "client"

	TableStatusFree = "free"
	TableStatusBusy = "busy"
)

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
	PricePerHour  int64
	CreatedAt     time.Time
	BranchName    string
}

type Session struct {
	ID         int64
	TableID    int64
	OperatorID int64
	ClientName string
	StartedAt  time.Time
	EndedAt    *time.Time
	TotalMin   int
	TotalPrice int64
	CreatedAt  time.Time
	TableNum   int
	BranchName string
}

func (s *Session) IsActive() bool { return s.EndedAt == nil }
func (s *Session) PriceSom() int64 { return s.TotalPrice / 100 }

func PricePerHourSom(p int64) int64 { return p / 100 }
