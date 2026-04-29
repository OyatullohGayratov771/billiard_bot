package service

import (
	"fmt"

	"table-service/internal/models"
	"table-service/internal/repository"
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

func (s *TableService) GetBranchByID(id int64) (*models.Branch, error) {
	return s.branchRepo.GetByID(id)
}

func (s *TableService) UpdateBranchNVR(id int64, ip string, port int, user, pass string) error {
	return s.branchRepo.UpdateNVR(id, ip, port, user, pass)
}

func (s *TableService) GetBranchTables(branchID int64) ([]*models.Table, error) {
	return s.tableRepo.GetByBranch(branchID)
}

func (s *TableService) GetTable(id int64) (*models.Table, error) {
	return s.tableRepo.GetByID(id)
}

func (s *TableService) SetTableRTSP(tableID int64, rtspURL string) error {
	return s.tableRepo.SetRTSPUrl(tableID, rtspURL)
}


// StartSession — operator role va branch id bilan tekshirib sessiya boshlaydi
func (s *TableService) StartSession(operatorID int64, operatorRole string, operatorBranchID *int64, tableID int64, clientName string) (*models.Session, error) {
	table, err := s.tableRepo.GetByID(tableID)
	if err != nil {
		return nil, fmt.Errorf("stol topilmadi")
	}

	if operatorRole == models.RoleOperator && operatorBranchID != nil && *operatorBranchID != table.BranchID {
		return nil, fmt.Errorf("siz faqat o'z fililingizdagi stollarni boshqara olasiz")
	}

	if table.Status == models.TableStatusBusy {
		return nil, fmt.Errorf("stol allaqachon band")
	}

	session, err := s.sessionRepo.Create(tableID, operatorID, clientName)
	if err != nil {
		return nil, err
	}

	if err := s.tableRepo.SetStatus(tableID, models.TableStatusBusy); err != nil {
		return nil, err
	}

	s.audit.Log(&operatorID, "start_session",
		fmt.Sprintf("table:%d client:%s session:%d", tableID, clientName, session.ID))

	return session, nil
}

// EndSession — sessiyani yakunlaydi
func (s *TableService) EndSession(operatorID int64, operatorRole string, operatorBranchID *int64, tableID int64) (*models.Session, error) {
	table, err := s.tableRepo.GetByID(tableID)
	if err != nil {
		return nil, fmt.Errorf("stol topilmadi")
	}

	if operatorRole == models.RoleOperator && operatorBranchID != nil && *operatorBranchID != table.BranchID {
		return nil, fmt.Errorf("siz faqat o'z fililingizdagi stollarni boshqara olasiz")
	}

	if table.Status == models.TableStatusFree {
		return nil, fmt.Errorf("stol allaqachon bo'sh")
	}

	session, err := s.sessionRepo.ActiveByTable(tableID)
	if err != nil {
		return nil, fmt.Errorf("faol sessiya topilmadi")
	}

	ended, err := s.sessionRepo.End(session.ID)
	if err != nil {
		return nil, err
	}

	if err := s.tableRepo.SetStatus(tableID, models.TableStatusFree); err != nil {
		return nil, err
	}

	s.audit.Log(&operatorID, "end_session",
		fmt.Sprintf("table:%d session:%d price:%d", tableID, session.ID, ended.TotalPrice/100))

	return ended, nil
}

func (s *TableService) ActiveSession(tableID int64) (*models.Session, error) {
	return s.sessionRepo.ActiveByTable(tableID)
}

func (s *TableService) DailyReport(branchID int64) (int, int64, error) {
	return s.sessionRepo.DailyReport(branchID)
}

func (s *TableService) MonthlyReport(branchID int64) (int, int64, error) {
	return s.sessionRepo.MonthlyReport(branchID)
}
