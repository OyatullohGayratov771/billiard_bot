package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// ProductClient talks to web-service's internal product endpoints (shared token).
type ProductClient struct {
	baseURL       string
	http          *http.Client
	internalToken string
}

func NewProductClient(baseURL string, httpClient *http.Client, internalToken string) *ProductClient {
	return &ProductClient{baseURL: baseURL, http: httpClient, internalToken: internalToken}
}

// Product mirrors the web-service product JSON.
type Product struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       int64  `json:"price"`
	Category    string `json:"category"`
	Emoji       string `json:"emoji"`
	ImageURL    string `json:"image_url"`
	InStock     bool   `json:"in_stock"`
	SortOrder   int    `json:"sort_order"`
}

func (c *ProductClient) auth(req *http.Request) {
	if c.internalToken != "" {
		req.Header.Set("X-Internal-Token", c.internalToken)
	}
}

// List returns all products (in-stock and hidden).
func (c *ProductClient) List() ([]*Product, error) {
	req, _ := http.NewRequest(http.MethodGet, c.baseURL+"/products/all", nil)
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("web-service: %d", resp.StatusCode)
	}
	var list []*Product
	return list, json.NewDecoder(resp.Body).Decode(&list)
}

// Create inserts a product and returns its id.
func (c *ProductClient) Create(p Product) (int64, error) {
	body, _ := json.Marshal(p)
	req, _ := http.NewRequest(http.MethodPost, c.baseURL+"/products", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return 0, fmt.Errorf("%s", e["error"])
	}
	var out struct {
		ID int64 `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.ID, nil
}

// UploadImage sends raw image bytes (multipart) and returns the stored URL.
func (c *ProductClient) UploadImage(data []byte, filename string) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("image", filename)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(fw, bytes.NewReader(data)); err != nil {
		return "", err
	}
	mw.Close()

	req, _ := http.NewRequest(http.MethodPost, c.baseURL+"/products/image", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return "", fmt.Errorf("%s", e["error"])
	}
	var out struct {
		URL string `json:"url"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.URL, nil
}

// Delete removes a product (and its image) by id.
func (c *ProductClient) Delete(id int64) error {
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/products/%d", c.baseURL, id), nil)
	c.auth(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("web-service: %d", resp.StatusCode)
	}
	return nil
}
