package models

type Branch struct {
	ID      int64
	Name    string
	NVRHost string
	NVRPort int
	NVRUser string
	NVRPass string
}

type Table struct {
	ID            int64
	BranchID      int64
	TableNum      int
	CameraChannel int
	RTSPUrl       string
}
