package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bot-gateway/internal/models"
)

type ClipClient struct {
	baseURL string
	http    *http.Client
}

func NewClipClient(baseURL string, httpClient *http.Client) *ClipClient {
	return &ClipClient{baseURL: baseURL, http: httpClient}
}

func (c *ClipClient) CreateRequest(in models.ClipRequestInput) (*models.ClipRequest, error) {
	body, _ := json.Marshal(map[string]any{
		"client_tg_id": in.ClientTgID,
		"client_name":  in.ClientName,
		"branch_id":    in.BranchID,
		"table_id":     in.TableID,
		"start_time":   in.StartTime.UTC().Format(time.RFC3339),
		"end_time":     in.EndTime.UTC().Format(time.RFC3339),
	})
	var cr models.ClipRequest
	if err := c.post("/clips", body, &cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

func (c *ClipClient) CreatePayment(clipID int64, screenshotFileID string) error {
	body, _ := json.Marshal(map[string]string{"screenshot_file_id": screenshotFileID})
	return c.postNoResponse(fmt.Sprintf("/clips/%d/payment", clipID), body)
}

func (c *ClipClient) ConfirmPayment(adminID, clipID int64) error {
	body, _ := json.Marshal(map[string]int64{"admin_id": adminID})
	return c.putNoResponse(fmt.Sprintf("/clips/%d/confirm", clipID), body)
}

func (c *ClipClient) SetStatus(adminID, clipID int64, status, note string) error {
	body, _ := json.Marshal(map[string]any{"admin_id": adminID, "status": status, "note": note})
	return c.putNoResponse(fmt.Sprintf("/clips/%d/status", clipID), body)
}

func (c *ClipClient) GetByID(id int64) (*models.ClipRequest, error) {
	var cr models.ClipRequest
	if err := c.get(fmt.Sprintf("/clips/%d", id), &cr); err != nil {
		return nil, err
	}
	return &cr, nil
}

func (c *ClipClient) ListByClient(tgID int64) ([]*models.ClipRequest, error) {
	var clips []*models.ClipRequest
	return clips, c.get(fmt.Sprintf("/clips/client/%d", tgID), &clips)
}

func (c *ClipClient) ListPending() ([]*models.ClipRequest, error) {
	var clips []*models.ClipRequest
	return clips, c.get("/clips/pending", &clips)
}

func (c *ClipClient) ListRecent(limit int) ([]*models.ClipRequest, error) {
	var clips []*models.ClipRequest
	return clips, c.get("/clips/recent", &clips)
}

func (c *ClipClient) TriggerRecording(clipID int64) error {
	return c.postNoResponse(fmt.Sprintf("/clips/%d/record", clipID), []byte("{}"))
}


func (c *ClipClient) DownloadClipFile(clipID int64) ([]byte, error) {
	resp, err := c.http.Get(fmt.Sprintf("%s/files/%d", c.baseURL, clipID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return nil, fmt.Errorf("clip-service: %s", e["error"])
	}
	var data []byte
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			data = append(data, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return data, nil
}

func (c *ClipClient) get(path string, out any) error {
	resp, err := c.http.Get(c.baseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("clip-service: %s", e["error"])
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *ClipClient) post(path string, body []byte, out any) error {
	resp, err := c.http.Post(c.baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("clip-service: %s", e["error"])
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *ClipClient) postNoResponse(path string, body []byte) error {
	resp, err := c.http.Post(c.baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("clip-service: %s", e["error"])
	}
	return nil
}

func (c *ClipClient) putNoResponse(path string, body []byte) error {
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
		return fmt.Errorf("clip-service: %s", e["error"])
	}
	return nil
}
