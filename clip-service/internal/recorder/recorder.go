package recorder

import (
	"fmt"
	"io"
	"net/http"
	"os"
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

// ISAPIDownloadClip — Hikvision NVR ISAPI HTTP orqali arxiv klipni yuklab oladi.
// RTSP o'rniga ishlatiladi — 401/500 xatolaridan xoli, ishonchli.
func (r *Recorder) ISAPIDownloadClip(clipID int64, nvrHost, nvrUser, nvrPass string, channel int, startTime, endTime time.Time) (string, error) {
	outPath := r.ClipPath(clipID)

	// Playback URI — NVR o'zining ichki RTSP manzili (HTTP orqali so'raladi)
	playbackURI := fmt.Sprintf(
		"rtsp://%s/Streaming/tracks/%d01?starttime=%sZ&endtime=%sZ",
		nvrHost, channel,
		startTime.UTC().Format("20060102T150405"),
		endTime.UTC().Format("20060102T150405"),
	)

	xmlBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<downloadRequest>
    <playbackURI>%s</playbackURI>
</downloadRequest>`, playbackURI)

	url := fmt.Sprintf("http://%s/ISAPI/ContentMgmt/download", nvrHost)

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(xmlBody))
	if err != nil {
		return "", fmt.Errorf("ISAPI request xatosi: %v", err)
	}
	req.Header.Set("Content-Type", "application/xml")
	req.SetBasicAuth(nvrUser, nvrPass)

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("NVR ga ulanishda xatolik: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("NVR xatosi %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	f, err := os.Create(outPath)
	if err != nil {
		return "", fmt.Errorf("fayl yaratishda xatolik: %v", err)
	}
	defer f.Close()

	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return "", fmt.Errorf("faylga yozishda xatolik: %v", err)
	}
	if written == 0 {
		return "", fmt.Errorf("NVR bo'sh javob qaytardi — o'sha vaqtda yozuv yo'q bo'lishi mumkin")
	}

	return outPath, nil
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
