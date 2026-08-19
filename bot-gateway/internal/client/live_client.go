package client

import (
	"encoding/json"
	"fmt"
	"net/http"
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
