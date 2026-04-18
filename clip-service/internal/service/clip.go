package service

import (
	"fmt"
	"log"
	"os"

	"clip-service/internal/models"
	"clip-service/internal/recorder"
	"clip-service/internal/repository"
)

type ClipService struct {
	clipRepo   *repository.ClipRepo
	branchRepo *repository.BranchRepo
	tableRepo  *repository.TableRepo
	audit      *repository.AuditRepo
	recorder   *recorder.Recorder
}

func NewClipService(
	clipRepo *repository.ClipRepo,
	branchRepo *repository.BranchRepo,
	tableRepo *repository.TableRepo,
	audit *repository.AuditRepo,
	rec *recorder.Recorder,
) *ClipService {
	return &ClipService{
		clipRepo:   clipRepo,
		branchRepo: branchRepo,
		tableRepo:  tableRepo,
		audit:      audit,
		recorder:   rec,
	}
}

func (s *ClipService) CreateRequest(in models.ClipRequestInput) (*models.ClipRequest, error) {
	cr, err := s.clipRepo.Create(in)
	if err != nil {
		return nil, err
	}
	s.audit.Log(nil, "clip_request_created",
		fmt.Sprintf("client:%d branch:%d table:%d start:%s end:%s",
			in.ClientTgID, in.BranchID, in.TableID,
			in.StartTime.Format("2006-01-02 15:04"),
			in.EndTime.Format("2006-01-02 15:04")))
	return cr, nil
}

func (s *ClipService) CreatePayment(clipID int64, screenshotFileID string) error {
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

func (s *ClipService) ConfirmPayment(adminID, clipID int64) error {
	if err := s.clipRepo.ConfirmPayment(clipID); err != nil {
		return err
	}
	s.audit.Log(&adminID, "payment_confirmed", fmt.Sprintf("clip:%d", clipID))
	return nil
}

func (s *ClipService) SetStatus(adminID, clipID int64, status, note string) error {
	if err := s.clipRepo.SetStatus(clipID, status, note); err != nil {
		return err
	}
	s.audit.Log(&adminID, "clip_status_changed",
		fmt.Sprintf("clip:%d status:%s note:%s", clipID, status, note))
	return nil
}

// RecordClip — NVR dan async klip yozadi
func (s *ClipService) RecordClip(clipID int64) error {
	cr, err := s.clipRepo.GetByID(clipID)
	if err != nil {
		return fmt.Errorf("buyurtma topilmadi")
	}
	if cr.Status != models.ClipStatusPaid && cr.Status != models.ClipStatusProcessing {
		return fmt.Errorf("klip yozish uchun avval to'lov tasdiqlanishi kerak (hozirgi holat: %s)", cr.Status)
	}

	table, err := s.tableRepo.GetByID(cr.TableID)
	if err != nil {
		return fmt.Errorf("stol topilmadi")
	}

	branch, err := s.branchRepo.GetByID(cr.BranchID)
	if err != nil {
		_ = s.clipRepo.SetStatus(clipID, models.ClipStatusFailed, "filial topilmadi")
		return fmt.Errorf("filial topilmadi")
	}

	// Kanal va NVR tekshirish
	useISAPI := branch.NVRHost != "" && table.CameraChannel > 0
	useRTSP := table.RTSPUrl != ""

	if !useISAPI && !useRTSP {
		_ = s.clipRepo.SetStatus(clipID, models.ClipStatusFailed, "")
		if branch.NVRHost == "" {
			return fmt.Errorf("filial NVR sozlanmagan (NVR IP kiriting)")
		}
		return fmt.Errorf("stol kamera kanali sozlanmagan")
	}

	// Status ni "processing" ga o'zgartir
	_ = s.clipRepo.SetStatus(clipID, models.ClipStatusProcessing, "")

	durationSec := int(cr.EndTime.Sub(cr.StartTime).Seconds())

	go func() {
		log.Printf("📥 Klip #%d yuklanmoqda: stol=%d kanal=%d [%s-%s] (%ds)",
			clipID, cr.TableID, table.CameraChannel,
			cr.StartTime.Format("15:04"), cr.EndTime.Format("15:04"), durationSec)

		var outPath string
		var recErr error

		if useISAPI {
			// Birlamchi: NVR ISAPI search + RTSP ffmpeg (Hikvision standart usuli)
			outPath, recErr = s.recorder.RecordFromNVR(
				clipID,
				branch.NVRHost, branch.NVRUser, branch.NVRPass,
				branch.NVRPort, table.CameraChannel,
				cr.StartTime, cr.EndTime,
			)
		} else {
			// Fallback: stol uchun alohida RTSP URL belgilangan
			rtspURL := recorder.PlaybackRTSP(table.RTSPUrl, cr.StartTime, cr.EndTime)
			outPath, recErr = s.recorder.Record(clipID, rtspURL, durationSec)
		}

		if recErr != nil {
			log.Printf("❌ Klip #%d xatolik: %v", clipID, recErr)
			_ = s.clipRepo.SetStatus(clipID, models.ClipStatusFailed, recErr.Error())
			_ = s.clipRepo.SetClipPath(clipID, "")
			return
		}

		_ = s.clipRepo.SetClipPath(clipID, outPath)
		_ = s.clipRepo.SetStatus(clipID, models.ClipStatusDone, "")
		log.Printf("✅ Klip #%d tayyor: %s", clipID, outPath)
	}()

	return nil
}

// TestCamera — NVR dan 1 ta kadr olib test qiladi
func (s *ClipService) TestCamera(branchID int64, channel int) (string, error) {
	branch, err := s.branchRepo.GetByID(branchID)
	if err != nil {
		return "", fmt.Errorf("filial topilmadi")
	}
	rtspURL := recorder.HikvisionRTSP(branch.NVRUser, branch.NVRPass, branch.NVRHost, branch.NVRPort, channel)
	if branch.NVRHost == "" {
		return "", fmt.Errorf("NVR sozlanmagan")
	}
	screenshotPath, err := s.recorder.Screenshot(0, rtspURL)
	if err != nil {
		return "", err
	}
	return screenshotPath, nil
}

// GetClipFile — klip faylini o'qib qaytaradi
func (s *ClipService) GetClipFile(clipID int64) ([]byte, string, error) {
	cr, err := s.clipRepo.GetByID(clipID)
	if err != nil {
		return nil, "", fmt.Errorf("buyurtma topilmadi")
	}
	if cr.ClipPath == "" || cr.Status != models.ClipStatusDone {
		return nil, "", fmt.Errorf("klip hali tayyor emas (holat: %s)", cr.Status)
	}

	data, err := os.ReadFile(cr.ClipPath)
	if err != nil {
		return nil, "", fmt.Errorf("fayl o'qishda xatolik: %v", err)
	}
	return data, cr.ClipPath, nil
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
