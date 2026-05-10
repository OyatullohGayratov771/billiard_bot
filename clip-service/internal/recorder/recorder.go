package recorder

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// tashkentTZ — NVR local vaqt zonasi (UTC+5). NVR "Z" UTC suffixini hisobga olmaydi.
var tashkentTZ = time.FixedZone("Asia/Tashkent", 5*60*60)

type Recorder struct {
	clipsDir string
}

func New(clipsDir string) *Recorder {
	return &Recorder{clipsDir: clipsDir}
}

func (r *Recorder) ClipPath(clipID int64) string {
	return filepath.Join(r.clipsDir, fmt.Sprintf("clip_%d.mp4", clipID))
}

// HikvisionPlaybackRTSP — NVR arxiv RTSP URL.
// Bu NVR "Z" ni UTC sifatida emas, majburiy suffix sifatida qabul qiladi — vaqt local.
func HikvisionPlaybackRTSP(user, pass, host string, port, channel int, startTime, endTime time.Time) string {
	if port == 0 {
		port = 554
	}
	return fmt.Sprintf(
		"rtsp://%s:%s@%s:%d/Streaming/tracks/%d01?starttime=%sZ&endtime=%sZ",
		user, pass, host, port, channel,
		startTime.In(tashkentTZ).Format("20060102T150405"),
		endTime.In(tashkentTZ).Format("20060102T150405"),
	)
}

// PlaybackRTSP — mavjud live RTSP URL dan playback URL yasaydi (RTSPUrl stollar uchun).
func PlaybackRTSP(liveURL string, startTime, endTime time.Time) string {
	playback := strings.Replace(liveURL, "/Streaming/Channels/", "/Streaming/tracks/", 1)
	return fmt.Sprintf("%s?starttime=%sZ&endtime=%sZ",
		playback,
		startTime.In(tashkentTZ).Format("20060102T150405"),
		endTime.In(tashkentTZ).Format("20060102T150405"),
	)
}

// HikvisionRTSP — live RTSP URL (TestCamera uchun).
func HikvisionRTSP(user, pass, host string, port, channel int) string {
	if port == 0 {
		port = 554
	}
	return fmt.Sprintf("rtsp://%s:%s@%s:%d/Streaming/Channels/%d01",
		user, pass, host, port, channel)
}




// runFFmpeg — ffmpeg ni timeout bilan ishga tushiradi.
func runFFmpeg(outPath string, timeout time.Duration, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(outPath)
		out := maskRTSP(strings.TrimSpace(stderr.String()))
		if ctx.Err() != nil {
			return fmt.Errorf("timeout(%v): %s", timeout, out)
		}
		return fmt.Errorf("%v | %s", err, out)
	}
	return nil
}

func logDone(clipID int64, durationSec int, outPath string) {
	sizeMB := 0.0
	if info, err := os.Stat(outPath); err == nil {
		sizeMB = float64(info.Size()) / 1024 / 1024
	}
	log.Printf("[klip#%d] done: %.1fMB, %ds → %s", clipID, sizeMB, durationSec, outPath)
}

// compressIfNeeded — har doim H.264 ga o'giradi (HEVC Telegram da ishlamaydi).
// Hajmni 40MB dan oshmaydigan qilib siqadi.
func compressIfNeeded(clipID int64, path string, durationSec int) error {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	sizeMB := float64(info.Size()) / 1024 / 1024
	// 38MB maqsad: audio (64kbps) ni hisobga olib video kbps hisoblash
	targetKbps := (38*1024*8 - 64*durationSec) / durationSec
	if targetKbps < 300 {
		targetKbps = 300
	}
	log.Printf("[klip#%d] H.264 ga o'girish: %.1fMB → maqsad %d kbps", clipID, sizeMB, targetKbps)

	tmpPath := path + ".tmp.mp4"
	err = runFFmpeg(tmpPath, time.Duration(durationSec*3+120)*time.Second, []string{
		"-loglevel", "warning",
		"-fflags", "+genpts+igndts+discardcorrupt",
		"-err_detect", "ignore_err",
		"-i", path,
		"-c:v", "libx264",
		"-b:v", fmt.Sprintf("%dk", targetKbps),
		"-maxrate", fmt.Sprintf("%dk", targetKbps*2),
		"-bufsize", fmt.Sprintf("%dk", targetKbps*4),
		"-preset", "ultrafast",
		"-threads", "2",
		"-c:a", "aac",
		"-ar", "44100",
		"-b:a", "64k",
		"-movflags", "+faststart",
		"-y", tmpPath,
	})
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("compress: %v", err)
	}

	os.Remove(path)
	if renErr := os.Rename(tmpPath, path); renErr != nil {
		return fmt.Errorf("rename: %v", renErr)
	}

	info2, err2 := os.Stat(path)
	if err2 != nil {
		return fmt.Errorf("siqilgan fayl topilmadi: %v", err2)
	}
	log.Printf("[klip#%d] siqildi: %.1fMB → %.1fMB", clipID, sizeMB, float64(info2.Size())/1024/1024)
	return nil
}

// RecordFromNVR — NVR arxividan RTSP orqali klip yozib oladi.
// Jarayon: ISAPI search (kesh) → RTSP -c copy (original sifat, decode yo'q)
func (r *Recorder) RecordFromNVR(clipID int64, nvrHost, nvrUser, nvrPass string, nvrPort, channel int, startTime, endTime time.Time) (string, error) {
	outPath := r.ClipPath(clipID)
	durationSec := int(endTime.Sub(startTime).Seconds())
	if durationSec <= 0 {
		return "", fmt.Errorf("noto'g'ri vaqt oralig'i")
	}

	log.Printf("[klip#%d] start: kanal=%d %s→%s (%ds)",
		clipID, channel, startTime.In(tashkentTZ).Format("15:04:05"), endTime.In(tashkentTZ).Format("15:04:05"), durationSec)

	path, err := r.recordRTSP(clipID, nvrHost, nvrUser, nvrPass, nvrPort, channel, startTime, endTime, durationSec, outPath)
	if err != nil {
		return "", err
	}
	if cErr := compressIfNeeded(clipID, path, durationSec); cErr != nil {
		os.Remove(path)
		return "", fmt.Errorf("siqishda xatolik: %v", cErr)
	}
	return path, nil
}

// maskRTSP — RTSP URL dagi parolni yashiradi (log uchun).
func maskRTSP(url string) string {
	// rtsp://user:pass@host/... → rtsp://user:***@host/...
	if i := strings.Index(url, "://"); i >= 0 {
		rest := url[i+3:]
		if at := strings.Index(rest, "@"); at >= 0 {
			creds := rest[:at]
			if colon := strings.Index(creds, ":"); colon >= 0 {
				return url[:i+3] + creds[:colon+1] + "***" + rest[at:]
			}
		}
	}
	return url
}

// recordRTSP — NVR RTSP orqali klip yozadi (H.264 encode — hamma telefon qo'llab-quvvatlaydi).
func (r *Recorder) recordRTSP(clipID int64, nvrHost, nvrUser, nvrPass string, nvrPort, channel int, startTime, endTime time.Time, durationSec int, outPath string) (string, error) {
	rtspURL := HikvisionPlaybackRTSP(nvrUser, nvrPass, nvrHost, nvrPort, channel, startTime, endTime)
	log.Printf("[klip#%d] RTSP: %s", clipID, maskRTSP(rtspURL))

	// -c:v copy: HEVC decode qilmaymiz (buzilgan stream muammolarini chetlab o'tish)
	// compressIfNeeded keyinchalik H.264 ga o'giradi
	// -stimeout: 25s inactivity timeout — CSeq mismatch/gap bo'lsa tez chiqadi
	timeout := time.Duration(durationSec+120) * time.Second

	err := runFFmpeg(outPath, timeout, []string{
		"-loglevel", "warning",
		"-fflags", "+genpts+igndts+discardcorrupt",
		"-rtsp_transport", "tcp",
		"-stimeout", "25000000",
		"-i", rtspURL,
		"-t", fmt.Sprintf("%d", durationSec),
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-c:v", "copy",
		"-c:a", "aac", "-ar", "44100", "-b:a", "64k",
		"-avoid_negative_ts", "make_zero",
		"-max_muxing_queue_size", "9999",
		"-movflags", "+faststart",
		"-y", outPath,
	})
	if err != nil {
		return "", fmt.Errorf("RTSP encode: %v", err)
	}

	logDone(clipID, durationSec, outPath)
	return outPath, nil
}

// Record — RTSPUrl bilan stollar uchun (-c copy, H.265 decode yo'q).
func (r *Recorder) Record(clipID int64, rtspURL string, durationSec int) (string, error) {
	outPath := r.ClipPath(clipID)
	args := []string{
		"-loglevel", "warning",
		"-fflags", "+genpts+igndts+discardcorrupt",
		"-err_detect", "ignore_err",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
	}
	if durationSec > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", durationSec))
	}
	args = append(args,
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-c:v", "copy",
		"-c:a", "aac", "-ar", "44100", "-b:a", "64k",
		"-avoid_negative_ts", "make_zero",
		"-max_muxing_queue_size", "9999",
		"-movflags", "+faststart",
		"-y", outPath,
	)
	timeout := time.Duration(durationSec+120) * time.Second
	if err := runFFmpeg(outPath, timeout, args); err != nil {
		return "", err
	}
	logDone(clipID, durationSec, outPath)
	if cErr := compressIfNeeded(clipID, outPath, durationSec); cErr != nil {
		os.Remove(outPath)
		return "", fmt.Errorf("siqishda xatolik: %v", cErr)
	}
	return outPath, nil
}

// Screenshot — RTSP streamdan 1 ta kadr oladi (NVR test uchun).
func (r *Recorder) Screenshot(clipID int64, rtspURL string) (string, error) {
	outPath := filepath.Join(r.clipsDir, fmt.Sprintf("test_%d.jpg", clipID))
	args := []string{
		"-loglevel", "warning",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-frames:v", "1",
		"-y", outPath,
	}
	if err := runFFmpeg(outPath, 30*time.Second, args); err != nil {
		return "", err
	}
	return outPath, nil
}
