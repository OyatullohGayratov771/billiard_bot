package recorder

import (
	"context"
	"fmt"
	"io"
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

// ClipPath — clip ID bo'yicha fayl yo'lini qaytaradi
func (r *Recorder) ClipPath(clipID int64) string {
	return filepath.Join(r.clipsDir, fmt.Sprintf("clip_%d.mp4", clipID))
}

// HikvisionPlaybackRTSP — NVR arxiv RTSP URL yasaydi (starttime/endtime bilan)
func HikvisionPlaybackRTSP(user, pass, host string, port, channel int, startTime, endTime time.Time) string {
	if port == 0 {
		port = 554
	}
	return fmt.Sprintf(
		"rtsp://%s:%s@%s:%d/Streaming/tracks/%d01?starttime=%sZ&endtime=%sZ",
		user, pass, host, port,
		channel,
		startTime.UTC().Format("20060102T150405"),
		endTime.UTC().Format("20060102T150405"),
	)
}

// PlaybackRTSP — mavjud live RTSP URL dan playback URL yasaydi (eski kod uchun)
func PlaybackRTSP(liveURL string, startTime, endTime time.Time) string {
	playback := strings.Replace(liveURL, "/Streaming/Channels/", "/Streaming/tracks/", 1)
	return fmt.Sprintf("%s?starttime=%sZ&endtime=%sZ",
		playback,
		startTime.UTC().Format("20060102T150405"),
		endTime.UTC().Format("20060102T150405"),
	)
}

// HikvisionRTSP — live RTSP URL (eski kod uchun saqlanadi)
func HikvisionRTSP(user, pass, host string, port, channel int) string {
	if port == 0 {
		port = 554
	}
	return fmt.Sprintf("rtsp://%s:%s@%s:%d/Streaming/Channels/%d01",
		user, pass, host, port, channel)
}

// searchWarmup — NVR da yozuv borligini tekshiradi va keshni yuklaydi.
// Digest auth ishlatadi. Bu qadam RTSP dan oldin kerak.
func searchWarmup(nvrHost, nvrUser, nvrPass string, channel int, startTime, endTime time.Time) {
	startStr := startTime.UTC().Format("2006-01-02T15:04:05Z")
	endStr := endTime.UTC().Format("2006-01-02T15:04:05Z")

	xmlBody := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>`+
		`<CMSearchDescription>`+
		`<searchID>B1LL1ARD-CLIP-SEARCH</searchID>`+
		`<trackIDList><trackID>%d01</trackID></trackIDList>`+
		`<timeSpanList><timeSpan>`+
		`<startTime>%s</startTime><endTime>%s</endTime>`+
		`</timeSpan></timeSpanList>`+
		`<maxResults>10</maxResults>`+
		`<searchResultPostion>0</searchResultPostion>`+
		`<metadataList><metadataDescriptor>//recordType.meta.std-cgi.com</metadataDescriptor></metadataList>`+
		`</CMSearchDescription>`,
		channel, startStr, endStr,
	)

	client := &http.Client{
		Transport: &digest.Transport{
			Username: nvrUser,
			Password: nvrPass,
		},
		Timeout: 30 * time.Second,
	}

	url := fmt.Sprintf("http://%s/ISAPI/ContentMgmt/search", nvrHost)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(xmlBody))
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

// RecordFromNVR — NVR arxividan klip yozib oladi:
//  1. ISAPI search orqali NVR keshini yuklaydi
//  2. RTSP + ffmpeg bilan yozadi (NVR o'zi endtime da stream ni to'xtatadi)
func (r *Recorder) RecordFromNVR(clipID int64, nvrHost, nvrUser, nvrPass string, nvrPort, channel int, startTime, endTime time.Time) (string, error) {
	outPath := r.ClipPath(clipID)

	durationSec := int(endTime.Sub(startTime).Seconds())
	if durationSec <= 0 {
		return "", fmt.Errorf("noto'g'ri vaqt oralig'i")
	}

	// 1-qadam: NVR keshini yuklash (search)
	searchWarmup(nvrHost, nvrUser, nvrPass, channel, startTime, endTime)

	// 2-qadam: RTSP URL yasash (starttime/endtime NVR ga beriladi — u o'zi to'g'ri vaqtda to'xtatadi)
	rtspURL := HikvisionPlaybackRTSP(nvrUser, nvrPass, nvrHost, nvrPort, channel, startTime, endTime)

	// 3-qadam: ffmpeg bilan yozish.
	// -t: NVR stream ni o'zi yopмайди — shu bilan aniq davomiylikda to'xtatamiz.
	// context: ffmpeg o'zi osilib qolsa — durationSec + 30 soniyada o'ldiriladi.
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(durationSec+30)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
		"-t", fmt.Sprintf("%d", durationSec),
		"-c:v", "copy", // video formatini o'zgartirmasdan nusxa olish
		"-an",          // pcm_mulaw audio MP4 da ishlamaydi
		"-y",
		outPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(outPath) // to'liq yozilмаган faylni o'chirish
		if ctx.Err() != nil {
			return "", fmt.Errorf("ffmpeg timeout: NVR stream %d sekunddan keyin ham tugamadi", durationSec+120)
		}
		return "", fmt.Errorf("ffmpeg xatosi: %v\n%s", err, string(output))
	}

	return outPath, nil
}

// Record — mavjud RTSP URL dan yozadi (eski/fallback usul, RTSPUrl bilan stollar uchun)
func (r *Recorder) Record(clipID int64, rtspURL string, durationSec int) (string, error) {
	outPath := r.ClipPath(clipID)

	args := []string{
		"-rtsp_transport", "tcp",
		"-i", rtspURL,
	}
	if durationSec > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", durationSec))
	}
	args = append(args, "-c", "copy", "-y", outPath)

	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("ffmpeg xatosi: %v\n%s", err, string(output))
	}
	return outPath, nil
}

// Screenshot — RTSP streamdan 1 ta kadr oladi (NVR test uchun)
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
