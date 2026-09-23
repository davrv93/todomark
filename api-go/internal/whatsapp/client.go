package whatsapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Client struct {
	enabled       bool
	baseURL       string
	token         string
	phoneNumberID string
	templateName  string
	language      string
	http          *http.Client
}

func NewFromEnv() *Client {
	return &Client{
		enabled:       strings.EqualFold(os.Getenv("USE_META_WHATSAPP"), "true"),
		baseURL:       envOr("META_WHATSAPP_URL", "https://graph.facebook.com/v16.0"),
		token:         os.Getenv("META_WHATSAPP_TOKEN"),
		phoneNumberID: os.Getenv("META_WHATSAPP_PHONE_NUMBER_ID"),
		templateName:  envOr("META_WHATSAPP_TEMPLATE", "ticket_update"),
		language:      envOr("META_WHATSAPP_LANGUAGE", "es"),
		http:          &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) Enabled() bool {
	return c.enabled
}

func (c *Client) SendText(number, text string) error {
	if !c.enabled {
		return errors.New("USE_META_WHATSAPP no habilitado")
	}
	if c.token == "" || c.phoneNumberID == "" {
		return errors.New("META_WHATSAPP_TOKEN o META_WHATSAPP_PHONE_NUMBER_ID no configurado")
	}
	body := map[string]any{
		"messaging_product": "whatsapp",
		"to":                number,
		"type":              "text",
		"text":              map[string]string{"body": text},
	}
	return c.do(body)
}

func (c *Client) SendTemplate(number string, params []string) error {
	if !c.enabled {
		return errors.New("USE_META_WHATSAPP no habilitado")
	}
	components := []map[string]any{}
	if len(params) > 0 {
		values := []map[string]string{}
		for _, p := range params {
			values = append(values, map[string]string{"type": "text", "text": p})
		}
		components = append(components, map[string]any{"type": "body", "parameters": values})
	}
	body := map[string]any{
		"messaging_product": "whatsapp",
		"to":                number,
		"type":              "template",
		"template": map[string]any{
			"name":       c.templateName,
			"language":   map[string]string{"code": c.language},
			"components": components,
		},
	}
	return c.do(body)
}

func (c *Client) do(body map[string]any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodPost, c.baseURL+"/"+c.phoneNumberID+"/messages", bytes.NewReader(raw))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.http.Do(req)
		if err != nil {
			last = err
		} else {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if resp.StatusCode < 300 {
				return nil
			}
			last = fmt.Errorf("meta whatsapp status %d: %s", resp.StatusCode, string(body))
			if resp.StatusCode != http.StatusTooManyRequests {
				return last
			}
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	return last
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
