package models

import "time"

const (
	RoleSuperadmin = "superadmin"
	RoleAdmin      = "admin"
	RoleOperator   = "operator"
	RoleClient     = "client"
)

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

type AuditLog struct {
	ID        int64
	UserID    *int64
	Action    string
	Details   string
	CreatedAt time.Time
}
