package recorder

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
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

// HikvisionPlaybackRTSP — NVR arxiv RTSP URL (RTSP fallback uchun).
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

// isapiDownload — NVR dan ISAPI HTTP orqali segment yuklab oladi.
//
// RTSP ga qaraganda afzalliklari:
//   - Real-time kutilmaydi — tarmoq tezligida yuklanadi
//   - H.265 decode qilinmaydi — raw stream saqlaydi
//   - RTSP port muammosi yo'q
func isapiDownload(client *http.Client, nvrHost string, channel int, startTime, endTime time.Time, destPath string) error {
	// NVR parser & va &amp; ni "tag" sifatida sanaydi → faqat starttime, endtime yo'q
	// ffmpeg -t bilan kerakli vaqtda to'xtatiladi
	playbackURI := fmt.Sprintf(
    "rtsp://%s/Streaming/tracks/%d01?starttime=%sZ&amp;endtime=%sZ",
    nvrHost, channel,
    startTime.UTC().Format("20060102T150405"),
    endTime.UTC().Format("20060102T150405"),
	)

	xmlBody := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<downloadRequest>` +
		`<playbackURI>` + playbackURI + `</playbackURI>` +
		`</downloadRequest>`

	log.Printf("[ISAPI] POST %s  body: %s", nvrHTTPBase(nvrHost)+"/ISAPI/ContentMgmt/download", xmlBody)

	req, err := http.NewRequest(http.MethodPost,
		nvrHTTPBase(nvrHost)+"/ISAPI/ContentMgmt/download",
		strings.NewReader(xmlBody))
	if err != nil {
		return fmt.Errorf("request: %v", err)
	}
	req.Header.Set("Content-Type", "application/xml")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connect: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("HTTP %s: %s", resp.Status, string(body))
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("file create: %v", err)
	}
	defer f.Close()

	ct := resp.Header.Get("Content-Type")
	mediaType, params, _ := mime.ParseMediaType(ct)

	// Ba'zi NVR lar multipart/mixed qaytaradi — video qismini ajratib olamiz
	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(resp.Body, params["boundary"])
		for {
			part, partErr := mr.NextPart()
			if partErr == io.EOF {
				break
			}
			if partErr != nil {
				return fmt.Errorf("multipart: %v", partErr)
			}
			partCT := part.Header.Get("Content-Type")
			if strings.Contains(partCT, "video") || strings.Contains(partCT, "octet-stream") || partCT == "" {
				_, err = io.Copy(f, part)
				return err
			}
			_, _ = io.Copy(io.Discard, part)
		}
		return fmt.Errorf("video part topilmadi multipart response da")
	}

	_, err = io.Copy(f, resp.Body)
	return err
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

// RecordFromNVR — ISAPI HTTP download orqali klip yozib oladi (asosiy usul).
//
// Jarayon:
//  1. ISAPI search  — NVR keshini yuklaydi
//  2. ISAPI download — HTTP orqali raw video yuklab oladi (H.265 decode yo'q)
//  3. ffmpeg -c copy  — faqat kesish (decode yo'q, juda tez)
//
// Fallback: ISAPI ishlamasa → RTSP -c copy (ham decode yo'q).
func (r *Recorder) RecordFromNVR(clipID int64, nvrHost, nvrUser, nvrPass string, nvrPort, channel int, startTime, endTime time.Time) (string, error) {
	outPath := r.ClipPath(clipID)

	durationSec := int(endTime.Sub(startTime).Seconds())
	if durationSec <= 0 {
		return "", fmt.Errorf("noto'g'ri vaqt oralig'i")
	}

	log.Printf("[klip#%d] start: kanal=%d %s→%s (%ds)",
		clipID, channel, startTime.Format("15:04:05"), endTime.Format("15:04:05"), durationSec)

	// ── 1-qadam: ISAPI search (NVR keshini yuklash) ──
	searchWarmup(nvrHost, nvrUser, nvrPass, channel, startTime, endTime)

	// ── 2-qadam: ISAPI HTTP download ──
	tmpPath := outPath + ".raw"
	dlClient := newDigestClient(nvrUser, nvrPass, time.Duration(durationSec+120)*time.Second)

	dlErr := isapiDownload(dlClient, nvrHost, channel, startTime, endTime, tmpPath)
	if dlErr != nil {
		log.Printf("[klip#%d] ISAPI xato: %v → RTSP fallback", clipID, dlErr)
		os.Remove(tmpPath)
		return r.recordRTSP(clipID, nvrHost, nvrUser, nvrPass, nvrPort, channel, startTime, endTime, durationSec, outPath, dlErr)
	}
	defer os.Remove(tmpPath)

	// ── 3-qadam: ffmpeg — faqat kesish, decode yo'q ──
	// -c:v copy: H.265/H.264 ni decode qilmasdan MP4 ga kesadi
	trimErr := runFFmpeg(outPath, 2*time.Minute, []string{
		"-loglevel", "warning",
		"-fflags", "+genpts",
		"-i", tmpPath,
		"-t", fmt.Sprintf("%d", durationSec),
		"-c:v", "copy",
		"-an",
		"-movflags", "+faststart",
		"-y", outPath,
	})
	if trimErr != nil {
		log.Printf("[klip#%d] ffmpeg kesish xato: %v → RTSP fallback", clipID, trimErr)
		return r.recordRTSP(clipID, nvrHost, nvrUser, nvrPass, nvrPort, channel, startTime, endTime, durationSec, outPath, trimErr)
	}

	logDone(clipID, durationSec, outPath)
	return outPath, nil
}

// recordRTSP — RTSP -c copy orqali yozadi (ISAPI ishlamagan holda fallback).
// H.265 decode qilinmaydi — copy mode ishlaydi.
func (r *Recorder) recordRTSP(clipID int64, nvrHost, nvrUser, nvrPass string, nvrPort, channel int, startTime, endTime time.Time, durationSec int, outPath string, prevErr error) (string, error) {
	rtspURL := HikvisionPlaybackRTSP(nvrUser, nvrPass, nvrHost, nvrPort, channel, startTime, endTime)
	log.Printf("[klip#%d] RTSP copy mode: %s", clipID, rtspURL)

	// NVR arxiv stream'ni boshlashga vaqt kerak (seek + 125ms latency) → timeout = duration*2+120
	rtspTimeout := time.Duration(durationSec*2+120) * time.Second
	err := runFFmpeg(outPath, rtspTimeout, []string{
		"-loglevel", "warning",
		"-fflags", "+genpts+igndts",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-t", fmt.Sprintf("%d", durationSec),
		"-c:v", "copy",
		"-an",
		"-movflags", "+faststart",
		"-y", outPath,
	})
	if err != nil {
		return "", fmt.Errorf("ISAPI: %v | RTSP: %v", prevErr, err)
	}

	logDone(clipID, durationSec, outPath)
	return outPath, nil
}

// Record — fallback: RTSPUrl bilan stollar uchun (-c copy, H.265 decode yo'q).
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
		"-an",
		"-movflags", "+faststart",
		"-y", outPath,
	)
	timeout := time.Duration(durationSec+60) * time.Second
	if err := runFFmpeg(outPath, timeout, args); err != nil {
		return "", err
	}
	logDone(clipID, durationSec, outPath)
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
