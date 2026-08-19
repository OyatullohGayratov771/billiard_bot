package streamer

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type stream struct {
	cancel    context.CancelFunc
	startedAt time.Time
}

// Manager — bir nechta stol uchun RTSP→HLS ffmpeg jarayonlarini boshqaradi.
// Holat faqat xotirada saqlanadi: servis qayta ishga tushsa (deploy/restart),
// ffmpeg bola-jarayonlar ham birga o'ladi, shuning uchun bazaga yozishning
// hojati yo'q — admin kerak bo'lsa botdan qayta yoqadi.
type Manager struct {
	mu      sync.Mutex
	streams map[int64]*stream
	hlsDir  string
}

func New(hlsDir string) *Manager {
	_ = os.MkdirAll(hlsDir, 0o755)
	return &Manager{streams: make(map[int64]*stream), hlsDir: hlsDir}
}

func (m *Manager) IsRunning(tableID int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.streams[tableID]
	return ok
}

func (m *Manager) ActiveTableIDs() []int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]int64, 0, len(m.streams))
	for id := range m.streams {
		ids = append(ids, id)
	}
	return ids
}

func (m *Manager) tableDir(tableID int64) string {
	return filepath.Join(m.hlsDir, strconv.FormatInt(tableID, 10))
}

// FilePath — HLS faylini (index.m3u8 yoki seg_*.ts) diskdan topish uchun.
func (m *Manager) FilePath(tableID int64, file string) string {
	return filepath.Join(m.tableDir(tableID), file)
}

// Start — stol uchun RTSP→HLS jarayonini boshlaydi. Allaqachon ishlab
// turgan bo'lsa hech narsa qilmaydi (idempotent).
func (m *Manager) Start(tableID int64, rtspURL string) error {
	m.mu.Lock()
	if _, ok := m.streams[tableID]; ok {
		m.mu.Unlock()
		return nil // allaqachon live
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.streams[tableID] = &stream{cancel: cancel, startedAt: time.Now()}
	m.mu.Unlock()

	dir := m.tableDir(tableID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.mu.Lock()
		delete(m.streams, tableID)
		m.mu.Unlock()
		cancel()
		return fmt.Errorf("papka yaratilmadi: %w", err)
	}
	cleanDir(dir)

	go m.runLoop(ctx, tableID, rtspURL, dir)
	log.Printf("📡 [stol#%d] live boshlandi", tableID)
	return nil
}

// Stop — stol uchun live oqimni to'xtatadi. Ishlamayotgan bo'lsa false qaytaradi.
func (m *Manager) Stop(tableID int64) bool {
	m.mu.Lock()
	st, ok := m.streams[tableID]
	if ok {
		delete(m.streams, tableID)
	}
	m.mu.Unlock()
	if !ok {
		return false
	}
	st.cancel()
	cleanDir(m.tableDir(tableID))
	log.Printf("⏹ [stol#%d] live to'xtatildi", tableID)
	return true
}

// StopAll — servis to'xtaganda (SIGTERM) barcha ffmpeg jarayonlarini tozalaydi.
func (m *Manager) StopAll() {
	for _, id := range m.ActiveTableIDs() {
		m.Stop(id)
	}
}

// runLoop — ffmpeg jarayonini ishga tushiradi; kutilmagan uzilishda
// (tarmoq/kamera muammosi) bir necha marta qayta urinadi.
func (m *Manager) runLoop(ctx context.Context, tableID int64, rtspURL, dir string) {
	const maxRetries = 5
	retries := 0
	for {
		if ctx.Err() != nil {
			return
		}
		err := runFFmpegHLS(ctx, rtspURL, dir)
		if ctx.Err() != nil {
			return // Stop() chaqirilgan — normal to'xtash
		}
		retries++
		reason := "oqim tugadi"
		if err != nil {
			reason = maskRTSP(err.Error())
		}
		log.Printf("⚠️ [stol#%d] live uzildi (%s), qayta urinish %d/%d", tableID, reason, retries, maxRetries)
		if retries >= maxRetries {
			log.Printf("❌ [stol#%d] live to'xtatildi — juda ko'p xatolik", tableID)
			m.mu.Lock()
			delete(m.streams, tableID)
			m.mu.Unlock()
			cleanDir(dir)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func cleanDir(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

// runFFmpegHLS — RTSP oqimini HLS (index.m3u8 + seg_*.ts) ga remux qiladi.
// "-c:v copy" — qayta kodlash yo'q, protsessorga juda yengil.
// Audio olib tashlanadi (-an) — kamerada mavjud bo'lmasligi/mos kelmasligi
// mumkin, live tomosha uchun video yetarli.
func runFFmpegHLS(ctx context.Context, rtspURL, dir string) error {
	args := []string{
		"-loglevel", "warning",
		"-rtsp_transport", "tcp",
		"-timeout", "15000000",
		"-i", rtspURL,
		"-an",
		"-c:v", "copy",
		"-f", "hls",
		"-hls_time", "2",
		"-hls_list_size", "6",
		"-hls_flags", "delete_segments+omit_endlist+independent_segments",
		"-hls_segment_filename", filepath.Join(dir, "seg_%03d.ts"),
		filepath.Join(dir, "index.m3u8"),
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil && ctx.Err() == nil {
		return fmt.Errorf("%v | %s", err, maskRTSP(strings.TrimSpace(stderr.String())))
	}
	return err
}

// maskRTSP — RTSP URL dagi parolni log'da yashiradi.
func maskRTSP(s string) string {
	i := strings.Index(s, "://")
	if i < 0 {
		return s
	}
	rest := s[i+3:]
	at := strings.Index(rest, "@")
	if at < 0 {
		return s
	}
	creds := rest[:at]
	colon := strings.Index(creds, ":")
	if colon < 0 {
		return s
	}
	return s[:i+3] + creds[:colon+1] + "***" + rest[at:]
}
