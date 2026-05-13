package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type WebClient struct {
	baseURL string
	http    *http.Client
}

func NewWebClient(baseURL string, httpClient *http.Client) *WebClient {
	return &WebClient{baseURL: baseURL, http: httpClient}
}

func (c *WebClient) CreateAuthURL(tgID int64, role string) (string, error) {
	body, _ := json.Marshal(map[string]any{"tg_id": tgID, "role": role})
	resp, err := c.http.Post(c.baseURL+"/internal/token", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var res struct {
		URL   string `json:"url"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if res.Error != "" {
		return "", fmt.Errorf(res.Error)
	}
	return res.URL, nil
}
