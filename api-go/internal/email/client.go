package email

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Client struct {
	endpoint string
	apiKey   string
	http     *http.Client
}

func NewClientFromEnv() *Client {
	return &Client{
		endpoint: os.Getenv("EMAIL_WEBHOOK_URL"),
		apiKey:   os.Getenv("EMAIL_API_KEY"),
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Send(to, subject, text string) error {
	if c.endpoint == "" {
		return errors.New("EMAIL_WEBHOOK_URL no configurado")
	}
	body, err := json.Marshal(map[string]string{"to": to, "subject": subject, "text": text})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("email provider status %d: %s", resp.StatusCode, string(raw))
	}
	return nil
}
