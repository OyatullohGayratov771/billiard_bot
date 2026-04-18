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

// HikvisionPlaybackRTSP — NVR arxiv RTSP URL.
// Format ishchi misol asosida: tracks/CHANNEL01/?starttime=...Z&endtime=...Z (trailing slash, port yo'q)
func HikvisionPlaybackRTSP(user, pass, host string, port, channel int, startTime, endTime time.Time) string {
	return fmt.Sprintf(
		"rtsp://%s:%s@%s/Streaming/tracks/%d01/?starttime=%sZ&endtime=%sZ",
		user, pass, host, channel,
		startTime.UTC().Format("20060102T150405"),
		endTime.UTC().Format("20060102T150405"),
	)
}

// PlaybackRTSP — mavjud live RTSP URL dan playback URL yasaydi (RTSPUrl stollar uchun).
func PlaybackRTSP(liveURL string, startTime, endTime time.Time) string {
	playback := strings.Replace(liveURL, "/Streaming/Channels/", "/Streaming/tracks/", 1)
	return fmt.Sprintf("%s?starttime=%sZ&endtime=%sZ",
		playback,
		startTime.UTC().Format("20060102T150405"),
		endTime.UTC().Format("20060102T150405"),
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

func newDigestClient(user, pass string, timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &digest.Transport{Username: user, Password: pass},
		Timeout:   timeout,
	}
}

// nvrHTTPBase — NVR HTTP base URL.
// host ga port qo'shish kerak bo'lsa (port 80 emas), host ni "ip:port" formatida bering.
func nvrHTTPBase(host string) string {
	return "http://" + host
}

// searchWarmup — NVR ISAPI search orqali arxiv keshini yuklaydi (download dan oldin kerak).
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
	client := newDigestClient(nvrUser, nvrPass, 30*time.Second)
	req, err := http.NewRequest(http.MethodPost,
		nvrHTTPBase(nvrHost)+"/ISAPI/ContentMgmt/search",
		strings.NewReader(xmlBody))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/xml")
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[NVR search] %s: %v", nvrHost, err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
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
		out := strings.TrimSpace(stderr.String())
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

const telegramMaxBytes = 45 * 1024 * 1024 // 45MB (Telegram 50MB limitidan xavfsiz chegarasi)

// compressIfNeeded — 45MB dan katta bo'lsa H.264 ga siqadi, kichik bo'lsa tegmaydi.
func compressIfNeeded(clipID int64, path string, durationSec int) error {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= telegramMaxBytes {
		return nil
	}

	sizeMB := float64(info.Size()) / 1024 / 1024
	// target 44MB: 44*1024*8 kbit / durationSec = kbps
	targetKbps := 44 * 1024 * 8 / durationSec
	if targetKbps < 300 {
		targetKbps = 300
	}
	log.Printf("[klip#%d] hajm %.1fMB > 45MB → siqilmoqda (%d kbps)", clipID, sizeMB, targetKbps)

	tmpPath := path + ".tmp.mp4"
	err = runFFmpeg(tmpPath, time.Duration(durationSec*3+120)*time.Second, []string{
		"-loglevel", "warning",
		"-i", path,
		"-c:v", "libx264",
		"-b:v", fmt.Sprintf("%dk", targetKbps),
		"-maxrate", fmt.Sprintf("%dk", targetKbps*2),
		"-bufsize", fmt.Sprintf("%dk", targetKbps*4),
		"-preset", "fast",
		"-c:a", "aac",
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

	if info2, err2 := os.Stat(path); err2 == nil {
		log.Printf("[klip#%d] siqildi: %.1fMB → %.1fMB", clipID, sizeMB, float64(info2.Size())/1024/1024)
	}
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
		clipID, channel, startTime.Format("15:04:05"), endTime.Format("15:04:05"), durationSec)

	searchWarmup(nvrHost, nvrUser, nvrPass, channel, startTime, endTime)
	path, err := r.recordRTSP(clipID, nvrHost, nvrUser, nvrPass, nvrPort, channel, startTime, endTime, durationSec, outPath)
	if err != nil {
		return "", err
	}
	if cErr := compressIfNeeded(clipID, path, durationSec); cErr != nil {
		log.Printf("[klip#%d] compress xato (original yuboriladi): %v", clipID, cErr)
	}
	return path, nil
}

// recordRTSP — RTSP -c copy orqali yozadi (ISAPI ishlamagan holda fallback).
func (r *Recorder) recordRTSP(clipID int64, nvrHost, nvrUser, nvrPass string, nvrPort, channel int, startTime, endTime time.Time, durationSec int, outPath string) (string, error) {
	rtspURL := HikvisionPlaybackRTSP(nvrUser, nvrPass, nvrHost, nvrPort, channel, startTime, endTime)
	log.Printf("[klip#%d] RTSP fallback: %s", clipID, rtspURL)

	timeout := time.Duration(durationSec*2+120) * time.Second
	err := runFFmpeg(outPath, timeout, []string{
		"-loglevel", "warning",
		"-fflags", "+genpts+igndts",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-t", fmt.Sprintf("%d", durationSec),
		"-c:v", "copy",
		"-c:a", "aac", "-b:a", "64k",
		"-movflags", "+faststart",
		"-y", outPath,
	})
	if err != nil {
		return "", fmt.Errorf("RTSP: %v", err)
	}

	logDone(clipID, durationSec, outPath)
	return outPath, nil
}

// Record — RTSPUrl bilan stollar uchun (-c copy, H.265 decode yo'q).
func (r *Recorder) Record(clipID int64, rtspURL string, durationSec int) (string, error) {
	outPath := r.ClipPath(clipID)
	args := []string{
		"-loglevel", "warning",
		"-fflags", "+genpts",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
	}
	if durationSec > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", durationSec))
	}
	args = append(args,
		"-c:v", "copy",
		"-c:a", "aac", "-b:a", "64k",
		"-movflags", "+faststart",
		"-y", outPath,
	)
	timeout := time.Duration(durationSec+60) * time.Second
	if err := runFFmpeg(outPath, timeout, args); err != nil {
		return "", err
	}
	logDone(clipID, durationSec, outPath)
	if cErr := compressIfNeeded(clipID, outPath, durationSec); cErr != nil {
		log.Printf("[klip#%d] compress xato (original yuboriladi): %v", clipID, cErr)
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
