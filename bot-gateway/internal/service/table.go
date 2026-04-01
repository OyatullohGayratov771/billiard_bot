package service

import (
	"fmt"

	"bot-gateway/internal/models"
	"bot-gateway/internal/repository"
)

type TableService struct {
	tableRepo   *repository.TableRepo
	branchRepo  *repository.BranchRepo
	sessionRepo *repository.SessionRepo
	audit       *repository.AuditRepo
}

func NewTableService(
	tableRepo *repository.TableRepo,
	branchRepo *repository.BranchRepo,
	sessionRepo *repository.SessionRepo,
	audit *repository.AuditRepo,
) *TableService {
	return &TableService{
		tableRepo:   tableRepo,
		branchRepo:  branchRepo,
		sessionRepo: sessionRepo,
		audit:       audit,
	}
}

func (s *TableService) GetBranches() ([]*models.Branch, error) {
	return s.branchRepo.GetAll()
}

func (s *TableService) GetBranchTables(branchID int64) ([]*models.Table, error) {
	return s.tableRepo.GetByBranch(branchID)
}

func (s *TableService) GetTable(id int64) (*models.Table, error) {
	return s.tableRepo.GetByID(id)
}

// StartSession — stolni band qiladi va sessiya yaratadi
func (s *TableService) StartSession(operator *models.User, tableID int64, clientName string) (*models.Session, error) {
	table, err := s.tableRepo.GetByID(tableID)
	if err != nil {
		return nil, fmt.Errorf("stol topilmadi")
	}

	// Operator o'z filialining stolini boshqara oladi
	if operator.IsOperator() && operator.BranchID != nil && *operator.BranchID != table.BranchID {
		return nil, fmt.Errorf("siz faqat o'z fililingizdagi stollarni boshqara olasiz")
	}

	if table.Status == models.TableStatusBusy {
		return nil, fmt.Errorf("stol allaqachon band")
	}

	// Sessiya yaratish
	session, err := s.sessionRepo.Create(tableID, operator.ID, clientName)
	if err != nil {
		return nil, err
	}

	// Stol holatini yangilash
	if err := s.tableRepo.SetStatus(tableID, models.TableStatusBusy); err != nil {
		return nil, err
	}

	s.audit.Log(&operator.ID, "start_session",
		fmt.Sprintf("table:%d client:%s session:%d", tableID, clientName, session.ID))

	return session, nil
}

// EndSession — stolni bo'sh qiladi va sessiyani yakunlaydi
func (s *TableService) EndSession(operator *models.User, tableID int64) (*models.Session, error) {
	table, err := s.tableRepo.GetByID(tableID)
	if err != nil {
		return nil, fmt.Errorf("stol topilmadi")
	}

	if operator.IsOperator() && operator.BranchID != nil && *operator.BranchID != table.BranchID {
		return nil, fmt.Errorf("siz faqat o'z fililingizdagi stollarni boshqara olasiz")
	}

	if table.Status == models.TableStatusFree {
		return nil, fmt.Errorf("stol allaqachon bo'sh")
	}

	// Faol sessiyani top
	session, err := s.sessionRepo.ActiveByTable(tableID)
	if err != nil {
		return nil, fmt.Errorf("faol sessiya topilmadi")
	}

	// Sessiyani yakunla
	ended, err := s.sessionRepo.End(session.ID)
	if err != nil {
		return nil, err
	}

	// Stol holatini yangilash
	if err := s.tableRepo.SetStatus(tableID, models.TableStatusFree); err != nil {
		return nil, err
	}

	s.audit.Log(&operator.ID, "end_session",
		fmt.Sprintf("table:%d session:%d price:%d", tableID, session.ID, ended.TotalPrice/100))

	return ended, nil
}

// ActiveSession — stolning hozirgi sessiyasini qaytaradi
func (s *TableService) ActiveSession(tableID int64) (*models.Session, error) {
	return s.sessionRepo.ActiveByTable(tableID)
}
