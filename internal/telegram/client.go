package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	apiURL string
	http   *http.Client
}

type SendMessageRequest struct {
	ChatID                string `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
	DisableNotification   bool   `json:"disable_notification,omitempty"`
}

type APIResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

func NewClient(apiURL string, timeout time.Duration) *Client {
	return &Client{
		apiURL: strings.TrimRight(apiURL, "/"),
		http:   &http.Client{Timeout: timeout},
	}
}

func (c *Client) SendMessage(ctx context.Context, token string, req SendMessageRequest) (*APIResponse, error) {
	if token == "" {
		return nil, fmt.Errorf("token is empty")
	}
	if req.ChatID == "" {
		return nil, fmt.Errorf("chat_id is empty")
	}
	if req.Text == "" {
		return nil, fmt.Errorf("text is empty")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	url := c.apiURL + "/bot" + token + "/sendMessage"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	limited, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var apiResp APIResponse
	if err := json.Unmarshal(limited, &apiResp); err != nil {
		return nil, fmt.Errorf("telegram returned non-json status=%d body=%q", resp.StatusCode, string(limited))
	}
	if resp.StatusCode >= 300 || !apiResp.OK {
		if apiResp.Description == "" {
			apiResp.Description = http.StatusText(resp.StatusCode)
		}
		return &apiResp, fmt.Errorf("telegram error status=%d code=%d description=%s", resp.StatusCode, apiResp.ErrorCode, apiResp.Description)
	}
	return &apiResp, nil
}
