package models

import "time"

const (
	ClipStatusPending    = "pending"
	ClipStatusPaid       = "paid"
	ClipStatusProcessing = "processing"
	ClipStatusDone       = "done"
	ClipStatusFailed     = "failed"
	ClipStatusRefunded   = "refunded"

	ClipPrice    = int64(1_000_000)
	ClipPriceSom = 10_000
)

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
	BranchName string
	TableNum   int
}

type ClipRequestInput struct {
	ClientTgID int64
	ClientName string
	BranchID   int64
	TableID    int64
	StartTime  time.Time
	EndTime    time.Time
}

// Branch — NVR ma'lumotlari uchun
type Branch struct {
	ID      int64
	Name    string
	NVRHost string
	NVRPort int
	NVRUser string
	NVRPass string
}

// Table — kamera kanali uchun
type Table struct {
	ID            int64
	BranchID      int64
	TableNum      int
	CameraChannel int
	RTSPUrl       string
}
