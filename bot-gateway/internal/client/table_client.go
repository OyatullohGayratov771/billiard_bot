package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"bot-gateway/internal/models"
)

type TableClient struct {
	baseURL string
	http    *http.Client
}

func NewTableClient(baseURL string, httpClient *http.Client) *TableClient {
	return &TableClient{baseURL: baseURL, http: httpClient}
}

func (c *TableClient) GetBranches() ([]*models.Branch, error) {
	var branches []*models.Branch
	return branches, c.get("/branches", &branches)
}

func (c *TableClient) GetBranchByID(id int64) (*models.Branch, error) {
	var branch models.Branch
	if err := c.get(fmt.Sprintf("/branches/%d", id), &branch); err != nil {
		return nil, err
	}
	return &branch, nil
}

func (c *TableClient) GetBranchTables(branchID int64) ([]*models.Table, error) {
	var tables []*models.Table
	return tables, c.get(fmt.Sprintf("/branches/%d/tables", branchID), &tables)
}

func (c *TableClient) GetTable(id int64) (*models.Table, error) {
	var table models.Table
	if err := c.get(fmt.Sprintf("/tables/%d", id), &table); err != nil {
		return nil, err
	}
	return &table, nil
}

func (c *TableClient) UpdateBranchNVR(id int64, ip string, port int, user, pass string) error {
	body, _ := json.Marshal(map[string]any{"ip": ip, "port": port, "user": user, "pass": pass})
	return c.putNoResponse(fmt.Sprintf("/branches/%d/nvr", id), body)
}

func (c *TableClient) SetTableRTSP(tableID int64, rtspURL string) error {
	body, _ := json.Marshal(map[string]string{"rtsp_url": rtspURL})
	return c.putNoResponse(fmt.Sprintf("/tables/%d/rtsp", tableID), body)
}

func (c *TableClient) SetTableChannel(tableID int64, channel int) error {
	body, _ := json.Marshal(map[string]int{"channel": channel})
	return c.putNoResponse(fmt.Sprintf("/tables/%d/channel", tableID), body)
}

func (c *TableClient) putNoResponse(path string, body []byte) error {
	req, err := http.NewRequest(http.MethodPut, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("table-service: %s", e["error"])
	}
	return nil
}

func (c *TableClient) get(path string, out any) error {
	resp, err := c.http.Get(c.baseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("table-service: %s", e["error"])
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *TableClient) post(path string, body []byte, out any) error {
	resp, err := c.http.Post(c.baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("table-service: %s", e["error"])
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

