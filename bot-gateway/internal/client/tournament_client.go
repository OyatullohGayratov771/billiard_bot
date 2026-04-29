package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bot-gateway/internal/models"
)

type TournamentClient struct {
	baseURL string
	http    *http.Client
}

func NewTournamentClient(baseURL string, httpClient *http.Client) *TournamentClient {
	return &TournamentClient{baseURL: baseURL, http: httpClient}
}

func (c *TournamentClient) CreateTournament(name string, branchID int64, tableID *int64, scheduledAt time.Time, price int64, maxPlayers int, adminTgID int64) (*models.Tournament, error) {
	body, _ := json.Marshal(map[string]any{
		"name":         name,
		"branch_id":    branchID,
		"table_id":     tableID,
		"scheduled_at": scheduledAt.Format(time.RFC3339),
		"price":        price,
		"max_players":  maxPlayers,
		"admin_tg_id":  adminTgID,
	})
	var t models.Tournament
	return &t, c.post("/tournaments", body, &t)
}

func (c *TournamentClient) ListTournaments(status string) ([]*models.Tournament, error) {
	var list []*models.Tournament
	url := "/tournaments"
	if status != "" {
		url += "?status=" + status
	}
	return list, c.get(url, &list)
}

func (c *TournamentClient) GetTournament(id int64) (*models.Tournament, error) {
	var t models.Tournament
	return &t, c.get(fmt.Sprintf("/tournaments/%d", id), &t)
}

func (c *TournamentClient) CancelTournament(id int64) error {
	return c.putNoResponse(fmt.Sprintf("/tournaments/%d/cancel", id), []byte("{}"))
}

func (c *TournamentClient) Register(tournamentID, userTgID int64, userName string) (*models.TournamentRegistration, error) {
	body, _ := json.Marshal(map[string]any{"user_tg_id": userTgID, "user_name": userName})
	var reg models.TournamentRegistration
	return &reg, c.post(fmt.Sprintf("/tournaments/%d/register", tournamentID), body, &reg)
}

func (c *TournamentClient) ListRegistrations(tournamentID int64) ([]*models.TournamentRegistration, error) {
	var list []*models.TournamentRegistration
	return list, c.get(fmt.Sprintf("/tournaments/%d/registrations", tournamentID), &list)
}

func (c *TournamentClient) ApproveRegistration(tournamentID, regID int64) error {
	return c.putNoResponse(fmt.Sprintf("/tournaments/%d/registrations/%d/approve", tournamentID, regID), []byte("{}"))
}

func (c *TournamentClient) RejectRegistration(tournamentID, regID int64) error {
	return c.putNoResponse(fmt.Sprintf("/tournaments/%d/registrations/%d/reject", tournamentID, regID), []byte("{}"))
}

func (c *TournamentClient) GenerateBracket(tournamentID int64) ([]*models.TournamentMatch, error) {
	var matches []*models.TournamentMatch
	return matches, c.post(fmt.Sprintf("/tournaments/%d/bracket", tournamentID), []byte("{}"), &matches)
}

func (c *TournamentClient) GetBracket(tournamentID int64) ([]*models.TournamentMatch, error) {
	var matches []*models.TournamentMatch
	return matches, c.get(fmt.Sprintf("/tournaments/%d/bracket", tournamentID), &matches)
}

func (c *TournamentClient) SetResult(matchID, winnerTgID int64) (nextMatchID int64, finished bool, err error) {
	body, _ := json.Marshal(map[string]any{"winner_tg_id": winnerTgID})
	var resp struct {
		NextMatchID int64 `json:"next_match_id"`
		Finished    bool  `json:"finished"`
	}
	if err := c.put(fmt.Sprintf("/matches/%d/result", matchID), body, &resp); err != nil {
		return 0, false, err
	}
	return resp.NextMatchID, resp.Finished, nil
}

func (c *TournamentClient) GetUserTournaments(tgID int64) ([]*models.TournamentRegistration, error) {
	var list []*models.TournamentRegistration
	return list, c.get(fmt.Sprintf("/users/%d/tournaments", tgID), &list)
}

// ===================== TV =====================

func (c *TournamentClient) GenerateTVToken(tournamentID int64) (string, error) {
	body, _ := json.Marshal(map[string]any{"tournament_id": tournamentID})
	var resp struct {
		Token string `json:"token"`
	}
	return resp.Token, c.post("/tv/token", body, &resp)
}

func (c *TournamentClient) GetTVTokenByTournament(tournamentID int64) (string, error) {
	var resp struct {
		Token string `json:"token"`
	}
	return resp.Token, c.get(fmt.Sprintf("/tv/by-tournament?tournament_id=%d", tournamentID), &resp)
}

func (c *TournamentClient) PushTVUpdate(token string) (int, error) {
	var resp struct {
		Viewers int `json:"viewers"`
	}
	body, _ := json.Marshal(map[string]any{})
	return resp.Viewers, c.post(fmt.Sprintf("/tv/%s/push", token), body, &resp)
}

func (c *TournamentClient) GetUserRegistration(tournamentID, userTgID int64) (*models.TournamentRegistration, error) {
	// Get all regs and filter — no dedicated endpoint needed
	regs, err := c.ListRegistrations(tournamentID)
	if err != nil {
		return nil, err
	}
	for _, r := range regs {
		if r.UserTgID == userTgID {
			return r, nil
		}
	}
	return nil, nil
}

func (c *TournamentClient) get(path string, out any) error {
	resp, err := c.http.Get(c.baseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("tournament-service: %s", e["error"])
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *TournamentClient) post(path string, body []byte, out any) error {
	resp, err := c.http.Post(c.baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("tournament-service: %s", e["error"])
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *TournamentClient) put(path string, body []byte, out any) error {
	req, _ := http.NewRequest(http.MethodPut, c.baseURL+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("tournament-service: %s", e["error"])
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *TournamentClient) putNoResponse(path string, body []byte) error {
	req, _ := http.NewRequest(http.MethodPut, c.baseURL+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("tournament-service: %s", e["error"])
	}
	return nil
}
