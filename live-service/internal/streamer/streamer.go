package streamer

import (
	"context"
	"errors"
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
	done      chan struct{}
	segments  []string // yozuv segmentlari (har bir ffmpeg urinishi o'zining faylini yozadi)
}

// recordingJob — Stop() dan keyin yozuvni birlashtirish/siqish fon jarayonining holati.
type recordingJob struct {
	mu       sync.Mutex
	ready    bool
	filename string
	err      error
}

// Manager — bir nechta stol uchun RTSP→HLS ffmpeg jarayonlarini boshqaradi.
// Holat faqat xotirada saqlanadi: servis qayta ishga tushsa (deploy/restart),
// ffmpeg bola-jarayonlar ham birga o'ladi, shuning uchun bazaga yozishning
// hojati yo'q — admin kerak bo'lsa botdan qayta yoqadi.
type Manager struct {
	mu         sync.Mutex
	streams    map[int64]*stream
	recordings map[int64]*recordingJob
	hlsDir     string
	recDir     string
}

func New(hlsDir, recDir string) *Manager {
	_ = os.MkdirAll(hlsDir, 0o755)
	_ = os.MkdirAll(recDir, 0o755)
	return &Manager{
		streams:    make(map[int64]*stream),
		recordings: make(map[int64]*recordingJob),
		hlsDir:     hlsDir,
		recDir:     recDir,
	}
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

// RecordingPath — yozuv faylini nomi bo'yicha diskdan topish uchun.
func (m *Manager) RecordingPath(filename string) string {
	return filepath.Join(m.recDir, filename)
}

// RecordingStatus — Stop()dan keyin yozuv tayyorlanish holatini (fon jarayoni
// tugaganmi, fayl nomi, xato) qaytaradi. exists=false bo'lsa bunday job umuman yo'q.
func (m *Manager) RecordingStatus(tableID int64) (ready bool, filename string, jobErr error, exists bool) {
	m.mu.Lock()
	job, ok := m.recordings[tableID]
	m.mu.Unlock()
	if !ok {
		return false, "", nil, false
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.ready, job.filename, job.err, true
}

// ClearRecording — yozuv muvaffaqiyatli yuborilgach (yoki kerak bo'lmay qolsa)
// job holatini va faylni tozalaydi.
func (m *Manager) ClearRecording(tableID int64) {
	m.mu.Lock()
	job, ok := m.recordings[tableID]
	if ok {
		delete(m.recordings, tableID)
	}
	m.mu.Unlock()
	if !ok {
		return
	}
	job.mu.Lock()
	fn := job.filename
	job.mu.Unlock()
	if fn != "" {
		_ = os.Remove(m.RecordingPath(fn))
	}
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
	st := &stream{cancel: cancel, startedAt: time.Now(), done: make(chan struct{})}
	m.streams[tableID] = st
	m.mu.Unlock()

	dir := m.tableDir(tableID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.mu.Lock()
		delete(m.streams, tableID)
		m.mu.Unlock()
		cancel()
		close(st.done)
		return fmt.Errorf("papka yaratilmadi: %w", err)
	}
	cleanDir(dir)

	go m.runLoop(ctx, tableID, rtspURL, dir, st)
	log.Printf("📡 [stol#%d] live boshlandi", tableID)
	return nil
}

// Stop — stol uchun live oqimni to'xtatadi va yozuvni fon jarayonida
// tayyorlashni boshlaydi (RecordingStatus orqali kuzatiladi). Ishlamayotgan
// bo'lsa false qaytaradi.
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
	<-st.done // ffmpeg to'liq to'xtaguncha kutamiz — oxirgi segment yozib bo'linsin
	cleanDir(m.tableDir(tableID))
	log.Printf("⏹ [stol#%d] live to'xtatildi", tableID)

	job := &recordingJob{}
	m.mu.Lock()
	m.recordings[tableID] = job
	m.mu.Unlock()
	go m.finalizeRecording(tableID, st, job)

	return true
}

// StopAll — servis to'xtaganda (SIGTERM) barcha ffmpeg jarayonlarini tozalaydi.
func (m *Manager) StopAll() {
	for _, id := range m.ActiveTableIDs() {
		m.Stop(id)
	}
}

// runLoop — ffmpeg jarayonini ishga tushiradi; kutilmagan uzilishda
// (tarmoq/kamera muammosi) bir necha marta qayta urinadi. Har bir urinish
// o'zining yozuv segment faylini ishlab chiqaradi (keyin birlashtiriladi).
func (m *Manager) runLoop(ctx context.Context, tableID int64, rtspURL, dir string, st *stream) {
	defer close(st.done)
	const maxRetries = 5
	retries := 0
	attempt := 0
	for {
		if ctx.Err() != nil {
			return
		}
		attempt++
		recPath := filepath.Join(m.recDir, fmt.Sprintf("%d_%d_%d.mp4", tableID, st.startedAt.Unix(), attempt))
		err := runFFmpegHLS(ctx, rtspURL, dir, recPath)
		if fi, statErr := os.Stat(recPath); statErr == nil && fi.Size() > 1024 {
			st.segments = append(st.segments, recPath)
		} else {
			_ = os.Remove(recPath) // bo'sh/muvaffaqiyatsiz urinish qoldig'i
		}
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

// finalizeRecording — Stop()dan keyin fon jarayonida ishlaydi: barcha urinish
// segmentlarini bitta faylga birlashtiradi, 50MB (Telegram bot cheklovi)dan
// oshsa past bitreytda qayta kodlaydi.
func (m *Manager) finalizeRecording(tableID int64, st *stream, job *recordingJob) {
	segs := st.segments // runLoop tugagan (done yopilgan), xavfsiz o'qish

	setErr := func(err error) {
		job.mu.Lock()
		job.err = err
		job.ready = true
		job.mu.Unlock()
	}
	setReady := func(filename string) {
		job.mu.Lock()
		job.filename = filename
		job.ready = true
		job.mu.Unlock()
	}

	if len(segs) == 0 {
		setErr(errors.New("yozuv topilmadi (efir judа qisqa bo'ldi yoki xato yuz berdi)"))
		return
	}

	final := filepath.Join(m.recDir, fmt.Sprintf("%d_%d.mp4", tableID, st.startedAt.Unix()))
	if len(segs) == 1 {
		if err := os.Rename(segs[0], final); err != nil {
			setErr(err)
			return
		}
	} else {
		if err := concatMP4(segs, final); err != nil {
			for _, s := range segs {
				_ = os.Remove(s)
			}
			setErr(err)
			return
		}
		for _, s := range segs {
			_ = os.Remove(s)
		}
	}

	fi, err := os.Stat(final)
	if err != nil {
		setErr(err)
		return
	}
	const maxBytes = 48 * 1024 * 1024 // 50MB Telegram limitidan xavfsizlik zaxirasi bilan kamroq
	if fi.Size() <= maxBytes {
		setReady(filepath.Base(final))
		return
	}

	duration, derr := probeDuration(final)
	if derr != nil || duration <= 0 {
		duration = 1800 // aniqlab bo'lmasa, ehtiyotkorlik uchun 30 daqiqa deb hisoblanadi
	}
	compressed := strings.TrimSuffix(final, ".mp4") + "_c.mp4"
	if err := compressMP4(final, compressed, maxBytes, duration); err != nil {
		log.Printf("⚠️ [stol#%d] yozuvni siqishda xato: %v — original hajmda qoldi", tableID, err)
		setReady(filepath.Base(final))
		return
	}
	_ = os.Remove(final)
	setReady(filepath.Base(compressed))
}

func cleanDir(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

// runFFmpegHLS — RTSP oqimini bir vaqtning o'zida ikki joyga yo'naltiradi:
// 1) HLS (index.m3u8 + seg_*.ts) — jonli tomosha uchun.
// 2) fragmentlangan MP4 — arxiv yozuvi uchun (jarayon kutilmaganda
//    o'chirilsa ham fayl buzilmasin deb "frag_keyframe+empty_moov" ishlatiladi).
// Ikkalasi ham "-c:v copy" — qayta kodlash yo'q, protsessorga juda yengil.
// Audio olib tashlanadi (-an) — kamerada mavjud bo'lmasligi/mos kelmasligi
// mumkin, video yetarli.
func runFFmpegHLS(ctx context.Context, rtspURL, dir, recPath string) error {
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
		"-an",
		"-c:v", "copy",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof",
		recPath,
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

// concatMP4 — bir nechta ffmpeg urinishidan qolgan segmentlarni (bir xil
// kodek parametrlari bilan) bitta faylga qayta kodlashsiz birlashtiradi.
func concatMP4(segs []string, out string) error {
	listFile := out + ".list.txt"
	var sb strings.Builder
	for _, s := range segs {
		sb.WriteString("file '" + s + "'\n")
	}
	if err := os.WriteFile(listFile, []byte(sb.String()), 0o644); err != nil {
		return err
	}
	defer os.Remove(listFile)
	cmd := exec.Command("ffmpeg", "-y", "-f", "concat", "-safe", "0", "-i", listFile, "-c", "copy", "-movflags", "+faststart", out)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("concat: %v | %s", err, stderr.String())
	}
	return nil
}

// probeDuration — video davomiyligini soniyalarda qaytaradi (siqish bitreytini hisoblash uchun).
func probeDuration(path string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

// compressMP4 — videoni berilgan hajm chegarasiga (maxBytes) sig'adigan
// bitreytda qayta kodlaydi (past o'lcham, 480p) — uzun yozuvlar Telegram
// bot cheklovi (50MB)ga sig'ishi uchun.
func compressMP4(in, out string, maxBytes int64, durationSec float64) error {
	budgetBits := float64(maxBytes) * 8 * 0.92 // xavfsizlik zaxirasi
	bitrateKbps := int(budgetBits / durationSec / 1000)
	if bitrateKbps < 80 {
		bitrateKbps = 80 // juda past bo'lib ko'rinmas holga tushmasin
	}
	cmd := exec.Command("ffmpeg", "-y", "-i", in,
		"-an", "-c:v", "libx264", "-preset", "veryfast",
		"-b:v", fmt.Sprintf("%dk", bitrateKbps),
		"-maxrate", fmt.Sprintf("%dk", bitrateKbps*12/10),
		"-bufsize", fmt.Sprintf("%dk", bitrateKbps*2),
		"-vf", "scale=-2:480",
		"-movflags", "+faststart",
		out,
	)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compress: %v | %s", err, stderr.String())
	}
	return nil
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
