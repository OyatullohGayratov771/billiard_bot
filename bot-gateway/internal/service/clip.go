package service

import (
	"fmt"

	"bot-gateway/internal/models"
	"bot-gateway/internal/repository"
)

type ClipService struct {
	clipRepo   *repository.ClipRepo
	branchRepo *repository.BranchRepo
	tableRepo  *repository.TableRepo
	audit      *repository.AuditRepo
}

func NewClipService(
	clipRepo *repository.ClipRepo,
	branchRepo *repository.BranchRepo,
	tableRepo *repository.TableRepo,
	audit *repository.AuditRepo,
) *ClipService {
	return &ClipService{
		clipRepo:   clipRepo,
		branchRepo: branchRepo,
		tableRepo:  tableRepo,
		audit:      audit,
	}
}

// CreateRequest — mijoz yangi klip so'rovi yaratadi
func (s *ClipService) CreateRequest(in models.ClipRequestInput) (*models.ClipRequest, error) {
	// filial va stol mavjudligini tekshir
	if _, err := s.branchRepo.GetByID(in.BranchID); err != nil {
		return nil, fmt.Errorf("filial topilmadi")
	}
	if _, err := s.tableRepo.GetByID(in.TableID); err != nil {
		return nil, fmt.Errorf("stol topilmadi")
	}

	cr, err := s.clipRepo.Create(in)
	if err != nil {
		return nil, err
	}

	s.audit.Log(nil, "clip_request_created",
		fmt.Sprintf("client:%d branch:%d table:%d time:%s",
			in.ClientTgID, in.BranchID, in.TableID, in.RequestedTime.Format("2006-01-02 15:04")))

	return cr, nil
}

// SubmitPaymentScreenshot — mijoz to'lov screenshotini yuboradi
func (s *ClipService) SubmitPaymentScreenshot(clipID int64, screenshotFileID string) error {
	cr, err := s.clipRepo.GetByID(clipID)
	if err != nil {
		return fmt.Errorf("buyurtma topilmadi")
	}
	if cr.Status != models.ClipStatusPending {
		return fmt.Errorf("buyurtma holati: %s — to'lov qabul qilinmaydi", cr.Status)
	}

	if err := s.clipRepo.CreatePayment(clipID, screenshotFileID); err != nil {
		return err
	}

	s.audit.Log(nil, "payment_screenshot",
		fmt.Sprintf("clip:%d screenshot:%s", clipID, screenshotFileID))
	return nil
}

// ConfirmPayment — admin to'lovni tasdiqlaydi
func (s *ClipService) ConfirmPayment(adminID, clipID int64) error {
	if err := s.clipRepo.ConfirmPayment(clipID); err != nil {
		return err
	}
	s.audit.Log(&adminID, "payment_confirmed", fmt.Sprintf("clip:%d", clipID))
	return nil
}

// SetStatus — admin klip statusini o'zgartiradi
func (s *ClipService) SetStatus(adminID, clipID int64, status string) error {
	if err := s.clipRepo.SetStatus(clipID, status); err != nil {
		return err
	}
	s.audit.Log(&adminID, "clip_status_changed",
		fmt.Sprintf("clip:%d status:%s", clipID, status))
	return nil
}

func (s *ClipService) GetByID(id int64) (*models.ClipRequest, error) {
	return s.clipRepo.GetByID(id)
}

func (s *ClipService) ListByClient(tgID int64) ([]*models.ClipRequest, error) {
	return s.clipRepo.ListByClient(tgID)
}

func (s *ClipService) ListPending() ([]*models.ClipRequest, error) {
	return s.clipRepo.ListPending()
}
