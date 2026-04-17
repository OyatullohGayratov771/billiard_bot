package recorder

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/icholy/digest"
)

type Recorder struct {
	clipsDir string
}

func New(clipsDir string) *Recorder {
	return &Recorder{clipsDir: clipsDir}
}

func (r *Recorder) ClipPath(clipID int64) string {
	return filepath.Join(r.clipsDir, fmt.Sprintf("clip_%d.mp4", clipID))
}

func HikvisionPlaybackRTSP(user, pass, host string, port, channel int, startTime, endTime time.Time) string {
	if port == 0 {
		port = 554
	}
	return fmt.Sprintf(
		"rtsp://%s:%s@%s:%d/Streaming/tracks/%d01?starttime=%sZ&endtime=%sZ",
		user, pass, host, port, channel,
		startTime.UTC().Format("20060102T150405"),
		endTime.UTC().Format("20060102T150405"),
	)
}

func PlaybackRTSP(liveURL string, startTime, endTime time.Time) string {
	playback := strings.Replace(liveURL, "/Streaming/Channels/", "/Streaming/tracks/", 1)
	return fmt.Sprintf("%s?starttime=%sZ&endtime=%sZ",
		playback,
		startTime.UTC().Format("20060102T150405"),
		endTime.UTC().Format("20060102T150405"),
	)
}

func HikvisionRTSP(user, pass, host string, port, channel int) string {
	if port == 0 {
		port = 554
	}
	return fmt.Sprintf("rtsp://%s:%s@%s:%d/Streaming/Channels/%d01",
		user, pass, host, port, channel)
}

// searchWarmup — NVR ISAPI search orqali arxiv keshini yuklaydi (Digest auth).
func searchWarmup(nvrHost, nvrUser, nvrPass string, channel int, startTime, endTime time.Time) {
	xmlBody := fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?>`+
			`<CMSearchDescription>`+
			`<searchID>BILLIARD-CLIP</searchID>`+
			`<trackIDList><trackID>%d01</trackID></trackIDList>`+
			`<timeSpanList><timeSpan>`+
			`<startTime>%s</startTime><endTime>%s</endTime>`+
			`</timeSpan></timeSpanList>`+
			`<maxResults>10</maxResults>`+
			`<searchResultPostion>0</searchResultPostion>`+
			`<metadataList><metadataDescriptor>//recordType.meta.std-cgi.com</metadataDescriptor></metadataList>`+
			`</CMSearchDescription>`,
		channel,
		startTime.UTC().Format("2006-01-02T15:04:05Z"),
		endTime.UTC().Format("2006-01-02T15:04:05Z"),
	)

	client := &http.Client{
		Transport: &digest.Transport{Username: nvrUser, Password: nvrPass},
		Timeout:   30 * time.Second,
	}
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("http://%s/ISAPI/ContentMgmt/search", nvrHost),
		strings.NewReader(xmlBody))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/xml")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

// runFFmpeg — ffmpeg ni timeout bilan ishga tushiradi, xato bo'lsa outPath o'chiriladi.
func runFFmpeg(outPath string, timeout time.Duration, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		os.Remove(outPath)
		out := strings.TrimSpace(stderr.String())
		if ctx.Err() != nil {
			return fmt.Errorf("timeout(%v): %s", timeout, out)
		}
		return fmt.Errorf("exit: %v | %s", err, out)
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

// RecordFromNVR — NVR arxividan klip yozib oladi.
// 1-urinish: decode + yadif (eng yaxshi sifat, interlace yo'q).
// 2-urinish: copy mode — agar HEVC stream buzilgan bo'lsa (CABAC/qp_delta xatolar).
func (r *Recorder) RecordFromNVR(clipID int64, nvrHost, nvrUser, nvrPass string, nvrPort, channel int, startTime, endTime time.Time) (string, error) {
	outPath := r.ClipPath(clipID)

	durationSec := int(endTime.Sub(startTime).Seconds())
	if durationSec <= 0 {
		return "", fmt.Errorf("noto'g'ri vaqt oralig'i")
	}

	searchWarmup(nvrHost, nvrUser, nvrPass, channel, startTime, endTime)
	rtspURL := HikvisionPlaybackRTSP(nvrUser, nvrPass, nvrHost, nvrPort, channel, startTime, endTime)

	log.Printf("[klip#%d] start: kanal=%d %s→%s (%ds)",
		clipID, channel, startTime.Format("15:04:05"), endTime.Format("15:04:05"), durationSec)

	// Telegram 50MB limitiga sig'adigan dinamik max bitrate (45MB target)
	maxBitrateKbps := 45 * 1024 * 8 / durationSec
	if maxBitrateKbps > 4000 {
		maxBitrateKbps = 4000
	}
	maxrateArg := fmt.Sprintf("%dk", maxBitrateKbps)

	// 1-urinish: decode + deinterlace (sifatli, lekin buzilgan HEVC da stall bo'lishi mumkin)
	decodeArgs := []string{
		"-loglevel", "warning",
		"-err_detect", "ignore_err",
		"-fflags", "+discardcorrupt+genpts",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-t", fmt.Sprintf("%d", durationSec),
		"-vf", "yadif=0:-1:0",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "20",
		"-maxrate", maxrateArg,
		"-bufsize", fmt.Sprintf("%dk", maxBitrateKbps*2),
		"-movflags", "+faststart",
		"-an", "-y", outPath,
	}
	decodeTimeout := time.Duration(durationSec+60) * time.Second
	if err := runFFmpeg(outPath, decodeTimeout, decodeArgs); err == nil {
		logDone(clipID, durationSec, outPath)
		return outPath, nil
	} else {
		log.Printf("[klip#%d] decode xato (%v), copy mode bilan qayta urinish...", clipID, err)
	}

	// 2-urinish: copy mode — HEVC decode qilinmaydi, stream to'g'ridan-to'g'ri yoziladi.
	// HEVC CABAC xatolar bu rejimda muammo qilmaydi.
	copyArgs := []string{
		"-loglevel", "warning",
		"-fflags", "+genpts",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-t", fmt.Sprintf("%d", durationSec),
		"-c:v", "copy",
		"-an", "-y", outPath,
	}
	copyTimeout := time.Duration(durationSec+30) * time.Second
	if err := runFFmpeg(outPath, copyTimeout, copyArgs); err != nil {
		log.Printf("[klip#%d] copy xato: %v", clipID, err)
		return "", fmt.Errorf("klip yozib bo'lmadi: %v", err)
	}

	logDone(clipID, durationSec, outPath)
	return outPath, nil
}

// Record — fallback: RTSPUrl bilan stollar uchun.
func (r *Recorder) Record(clipID int64, rtspURL string, durationSec int) (string, error) {
	outPath := r.ClipPath(clipID)

	args := []string{
		"-loglevel", "warning",
		"-err_detect", "ignore_err",
		"-fflags", "+discardcorrupt+genpts",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
	}
	if durationSec > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", durationSec))
	}
	args = append(args,
		"-vf", "yadif=0:-1:0",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "20",
		"-movflags", "+faststart",
		"-an", "-y", outPath,
	)

	timeout := time.Duration(durationSec+60) * time.Second
	if err := runFFmpeg(outPath, timeout, args); err != nil {
		return "", err
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
	timeout := 30 * time.Second
	if err := runFFmpeg(outPath, timeout, args); err != nil {
		return "", err
	}
	return outPath, nil
}
