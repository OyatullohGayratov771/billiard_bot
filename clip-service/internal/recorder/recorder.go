package recorder

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Recorder struct {
	clipsDir string
}

func New(clipsDir string) *Recorder {
	return &Recorder{clipsDir: clipsDir}
}

// HikvisionRTSP — Hikvision NVR uchun RTSP URL yasaydi
// channel 1 → Streaming/Channels/101, channel 2 → 201, ...
func HikvisionRTSP(user, pass, host string, port, channel int) string {
	if port == 0 {
		port = 554
	}
	return fmt.Sprintf("rtsp://%s:%s@%s:%d/Streaming/Channels/%d01",
		user, pass, host, port, channel)
}

// PlaybackRTSP — live RTSP URL dan Hikvision playback URL yasaydi
// rtsp://user:pass@ip:554/Streaming/Channels/101
// → rtsp://user:pass@ip:554/Streaming/tracks/101?starttime=...&endtime=...
func PlaybackRTSP(liveURL string, startTime, endTime time.Time) string {
	playback := strings.Replace(liveURL, "/Streaming/Channels/", "/Streaming/tracks/", 1)
	return fmt.Sprintf("%s?starttime=%sZ&endtime=%sZ",
		playback,
		startTime.UTC().Format("20060102T150405"),
		endTime.UTC().Format("20060102T150405"),
	)
}

// DahuaRTSP — Dahua NVR uchun RTSP URL yasaydi
func DahuaRTSP(user, pass, host string, port, channel int) string {
	if port == 0 {
		port = 554
	}
	return fmt.Sprintf("rtsp://%s:%s@%s:%d/cam/realmonitor?channel=%d&subtype=0",
		user, pass, host, port, channel)
}

// ClipPath — clip ID bo'yicha fayl yo'lini qaytaradi
func (r *Recorder) ClipPath(clipID int64) string {
	return filepath.Join(r.clipsDir, fmt.Sprintf("clip_%d.mp4", clipID))
}

// Record — FFmpeg orqali Hikvision playback RTSP dan video oladi (bloklaydi)
// rtspURL da starttime/endtime bor — kamera o'zi endtime da to'xtatadi
func (r *Recorder) Record(clipID int64, rtspURL string) (string, error) {
	outPath := r.ClipPath(clipID)

	cmd := exec.Command("ffmpeg",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-c:v", "copy",
		"-an",         // audio yo'q
		"-y",          // mavjud faylni ustiga yoz
		outPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg xatosi: %v\n%s", err, string(output))
	}

	return outPath, nil
}

// Screenshot — RTSP streamdan 1 ta kadr oladi (test uchun)
func (r *Recorder) Screenshot(clipID int64, rtspURL string) (string, error) {
	outPath := filepath.Join(r.clipsDir, fmt.Sprintf("test_%d.jpg", clipID))

	cmd := exec.Command("ffmpeg",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-frames:v", "1",
		"-y",
		outPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg xatosi: %v\n%s", err, string(output))
	}

	return outPath, nil
}
