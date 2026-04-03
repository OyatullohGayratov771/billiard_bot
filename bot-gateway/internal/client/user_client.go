package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"bot-gateway/internal/models"
)

type UserClient struct {
	baseURL string
	http    *http.Client
}

func NewUserClient(baseURL string) *UserClient {
	return &UserClient{baseURL: baseURL, http: &http.Client{}}
}

func (c *UserClient) RegisterOrGet(tgID int64, username, firstName, lastName string) (*models.User, error) {
	body, _ := json.Marshal(map[string]any{
		"tg_id":      tgID,
		"username":   username,
		"first_name": firstName,
		"last_name":  lastName,
	})
	var user models.User
	if err := c.post("/users/upsert", body, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *UserClient) GetByTelegramID(tgID int64) (*models.User, error) {
	var user models.User
	if err := c.get(fmt.Sprintf("/users/tg/%d", tgID), &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *UserClient) ListByRole(role string) ([]*models.User, error) {
	var users []*models.User
	if err := c.get(fmt.Sprintf("/users/role/%s", role), &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (c *UserClient) ListStaff() ([]*models.User, error) {
	var users []*models.User
	if err := c.get("/users/all", &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (c *UserClient) SetRole(adminTgID, targetTgID int64, role string, branchID *int64) error {
	body, _ := json.Marshal(map[string]any{
		"admin_tg_id": adminTgID,
		"role":        role,
		"branch_id":   branchID,
	})
	return c.putNoResponse(fmt.Sprintf("/users/tg/%d/role", targetTgID), body)
}

func (c *UserClient) get(path string, out any) error {
	resp, err := c.http.Get(c.baseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("user-service: %s", e["error"])
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *UserClient) post(path string, body []byte, out any) error {
	resp, err := c.http.Post(c.baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("user-service: %s", e["error"])
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *UserClient) putNoResponse(path string, body []byte) error {
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
		return fmt.Errorf("user-service: %s", e["error"])
	}
	return nil
}
