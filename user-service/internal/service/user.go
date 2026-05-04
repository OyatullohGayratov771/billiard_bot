package service

import (
	"fmt"

	"user-service/internal/models"
	"user-service/internal/repository"
)

type UserService struct {
	repo        *repository.UserRepo
	audit       *repository.AuditRepo
	superadmins []int64
}

func NewUserService(repo *repository.UserRepo, audit *repository.AuditRepo, superadmins []int64) *UserService {
	return &UserService{repo: repo, audit: audit, superadmins: superadmins}
}

func (s *UserService) RegisterOrGet(tgID int64, username, firstName, lastName string) (*models.User, error) {
	user, err := s.repo.Upsert(tgID, username, firstName, lastName)
	if err != nil {
		return nil, err
	}
	if s.isSuperadmin(tgID) && user.Role != models.RoleSuperadmin {
		if err := s.repo.SetRole(tgID, models.RoleSuperadmin, nil); err != nil {
			return nil, err
		}
		user.Role = models.RoleSuperadmin
		s.audit.Log(&user.ID, "auto_superadmin", fmt.Sprintf("tg:%d", tgID))
	}
	return user, nil
}

func (s *UserService) GetByTelegramID(tgID int64) (*models.User, error) {
	return s.repo.GetByTelegramID(tgID)
}

func (s *UserService) SetRole(adminTgID, targetTgID int64, role string, branchID *int64) error {
	admin, err := s.repo.GetByTelegramID(adminTgID)
	if err != nil {
		return fmt.Errorf("admin topilmadi")
	}
	if !admin.IsSuperadmin() {
		return fmt.Errorf("faqat superadmin role belgilashi mumkin")
	}
	target, err := s.repo.GetByTelegramID(targetTgID)
	if err != nil {
		return fmt.Errorf("foydalanuvchi topilmadi: %d", targetTgID)
	}
	if err := s.repo.SetRole(targetTgID, role, branchID); err != nil {
		return err
	}
	s.audit.Log(&admin.ID, "set_role",
		fmt.Sprintf("target:%d role:%s branch:%v", target.ID, role, branchID))
	return nil
}

func (s *UserService) ListStaff() ([]*models.User, error) {
	return s.repo.ListAll()
}

func (s *UserService) ListByRole(role string) ([]*models.User, error) {
	return s.repo.ListByRole(role)
}

func (s *UserService) SavePhone(tgID int64, phone string) error {
	return s.repo.SavePhone(tgID, phone)
}

func (s *UserService) UpdateName(tgID int64, firstName string) error {
	return s.repo.UpdateName(tgID, firstName)
}

func (s *UserService) isSuperadmin(tgID int64) bool {
	for _, id := range s.superadmins {
		if id == tgID {
			return true
		}
	}
	return false
}
