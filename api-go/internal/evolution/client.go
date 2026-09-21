package evolution

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
	baseURL  string
	apiKey   string
	instance string
	http     *http.Client
}

func NewFromEnv() *Client {
	return &Client{
		baseURL:  envOr("EVOLUTION_URL", "http://localhost:3100"),
		apiKey:   os.Getenv("EVOLUTION_API_KEY"),
		instance: envOr("EVOLUTION_INSTANCE", "todomark"),
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ErrorHTTP preserva el código de estado de Evolution para mapear 502/404 en el gateway.
type ErrorHTTP struct {
	Status int
	Body   string
}

func (e *ErrorHTTP) Error() string {
	return fmt.Sprintf("evolution status %d: %s", e.Status, e.Body)
}

func (c *Client) do(method, path string, body any) (map[string]any, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, &ErrorHTTP{Status: resp.StatusCode, Body: string(raw)}
	}
	var out map[string]any
	if len(raw) > 0 {
		json.Unmarshal(raw, &out)
	}
	return out, nil
}

func (c *Client) SendMessage(number, text string) error {
	_, err := c.do("POST", "/message/sendText/"+c.instance, map[string]any{
		"number": number, "text": text,
	})
	return err
}

func (c *Client) ConnectionStatus() (string, error) {
	data, err := c.do("GET", "/instance/connectionState/"+c.instance, nil)
	if err != nil {
		return "", err
	}
	if inst, ok := data["instance"].(map[string]any); ok {
		if s, ok := inst["state"].(string); ok {
			return s, nil
		}
	}
	return "unknown", nil
}

// Connect devuelve el QR (base64 o code), o nil si ya está conectada.
func (c *Client) Connect() (*string, error) {
	data, err := c.do("GET", "/instance/connect/"+c.instance, nil)
	if err != nil {
		return nil, err
	}
	if q, ok := data["qrcode"].(map[string]any); ok {
		if b, ok := q["base64"].(string); ok && b != "" {
			return &b, nil
		}
		if code, ok := q["code"].(string); ok && code != "" {
			code = "QR: " + code
			return &code, nil
		}
		return nil, nil
	}
	if b, ok := data["base64"].(string); ok && b != "" {
		return &b, nil
	}
	return nil, nil
}

// Logout desvincula el número de WhatsApp sin borrar la instancia.
func (c *Client) Logout() error {
	_, err := c.do("DELETE", "/instance/logout/"+c.instance, nil)
	return err
}

func (c *Client) CreateInstance() error {
	_, err := c.do("POST", "/instance/create", map[string]any{
		"instanceName": c.instance,
		"integration":  "WHATSAPP-BAILEYS",
		"qrcode":       true,
	})
	return err
}

// PairingCode pide un código de 8 caracteres para vincular sin escanear QR.
// Requiere una sesión de emparejamiento activa (haber generado QR justo antes).
func (c *Client) PairingCode(phone string) (string, error) {
	data, err := c.do("GET", "/instance/connect/"+c.instance+"?number="+phone, nil)
	if err != nil {
		return "", err
	}
	if q, ok := data["qrcode"].(map[string]any); ok {
		if pc, ok := q["pairingCode"].(string); ok && pc != "" {
			return pc, nil
		}
	}
	if pc, ok := data["pairingCode"].(string); ok && pc != "" {
		return pc, nil
	}
	return "", errors.New("Evolution no devolvió pairing code; genera un QR primero y reintenta")
}

// SetWebhook registra el webhook de mensajes entrantes en la instancia.
// En v2.3.7 `events` es un array de nombres de evento.
func (c *Client) SetWebhook(url string) error {
	_, err := c.do("POST", "/webhook/set/"+c.instance, map[string]any{
		"webhook": map[string]any{
			"enabled": true,
			"url":     url,
			"events":  []string{"MESSAGES_UPSERT"},
		},
	})
	return err
}
