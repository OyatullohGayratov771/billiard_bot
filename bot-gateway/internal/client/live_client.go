package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LiveClient — live-service ning internal (X-Internal-Token) endpointlariga murojaat qiladi.
type LiveClient struct {
	baseURL       string
	http          *http.Client
	internalToken string
}

func NewLiveClient(baseURL string, httpClient *http.Client, internalToken string) *LiveClient {
	return &LiveClient{baseURL: baseURL, http: httpClient, internalToken: internalToken}
}

func (c *LiveClient) auth(req *http.Request) {
	if c.internalToken != "" {
		req.Header.Set("X-Internal-Token", c.internalToken)
	}
}

type LiveStartResult struct {
	LiveURL  string `json:"live_url"`
	TableNum int    `json:"table_num"`
}

// Start — stol uchun jonli efirni yoqadi (idempotent — allaqachon yoniq bo'lsa xato bermaydi).
func (c *LiveClient) Start(tableID int64) (*LiveStartResult, error) {
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/streams/%d/start", c.baseURL, tableID), nil)
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e["error"] != "" {
			return nil, fmt.Errorf("%s", e["error"])
		}
		return nil, fmt.Errorf("live-service: %d", resp.StatusCode)
	}
	var out LiveStartResult
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return &out, nil
}

// Stop — stol uchun jonli efirni o'chiradi.
func (c *LiveClient) Stop(tableID int64) error {
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/streams/%d/stop", c.baseURL, tableID), nil)
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("live-service: %d", resp.StatusCode)
	}
	return nil
}

// RecordingStatus — Stop()dan keyingi yozuv tayyorlik holati.
type RecordingStatus struct {
	Ready    bool   `json:"ready"`
	Filename string `json:"filename"`
	Error    string `json:"error"`
	Exists   bool   `json:"-"`
}

// RecordingStatus — yozuv (kanalga yuborish uchun) tayyor bo'lganini so'raydi.
func (c *LiveClient) RecordingStatus(tableID int64) (*RecordingStatus, error) {
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/recordings/%d/status", c.baseURL, tableID), nil)
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return &RecordingStatus{Exists: false}, nil
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("live-service: %d", resp.StatusCode)
	}
	var out RecordingStatus
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	out.Exists = true
	return &out, nil
}

// DownloadRecording — tayyor bo'lgan yozuv faylini yuklab oladi (Telegram'ga
// qayta yuborish uchun). Katta fayl bo'lishi mumkinligi uchun uzunroq timeout ishlatiladi.
func (c *LiveClient) DownloadRecording(tableID int64) ([]byte, error) {
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/recordings/%d/download", c.baseURL, tableID), nil)
	c.auth(req)
	dlClient := &http.Client{Timeout: 120 * time.Second}
	resp, err := dlClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("live-service: %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// DeleteRecording — Telegram'ga muvaffaqiyatli yuborilgach yozuvni tozalaydi.
func (c *LiveClient) DeleteRecording(tableID int64) error {
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/recordings/%d", c.baseURL, tableID), nil)
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// Active — hozir jonli bo'lgan stol ID'lari (tableID -> true).
func (c *LiveClient) Active() (map[int64]bool, error) {
	req, _ := http.NewRequest(http.MethodGet, c.baseURL+"/streams", nil)
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("live-service: %d", resp.StatusCode)
	}
	var ids []int64
	if err := json.NewDecoder(resp.Body).Decode(&ids); err != nil {
		return nil, err
	}
	out := make(map[int64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}
