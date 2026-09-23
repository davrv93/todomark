package evolution

import (
	"bytes"
	"encoding/base64"
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
	baseURL   string
	apiKey    string
	instance  string
	http      *http.Client
	allowlist map[string]bool // si no está vacío, SOLO se puede mandar a estos números
}

func NewFromEnv() *Client {
	c := &Client{
		baseURL:  envOr("EVOLUTION_URL", "http://localhost:3100"),
		apiKey:   os.Getenv("EVOLUTION_API_KEY"),
		instance: envOr("EVOLUTION_INSTANCE", "todomark"),
		http:     &http.Client{Timeout: 15 * time.Second},
	}
	// WHATSAPP_ALLOWLIST: candado opcional (ej. mientras el demo está expuesto por túnel
	// a gente fuera del equipo) — número(s) separados por coma, sin "+". Vacío = sin restricción
	// (comportamiento normal). Ver DOCUMENTACION.md / memoria "whatsapp-test-number-only".
	if raw := strings.TrimSpace(os.Getenv("WHATSAPP_ALLOWLIST")); raw != "" {
		c.allowlist = map[string]bool{}
		for _, n := range strings.Split(raw, ",") {
			n = onlyDigits(strings.TrimSpace(n))
			if n != "" {
				c.allowlist[n] = true
			}
		}
	}
	return c
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// allowed valida el destino contra WHATSAPP_ALLOWLIST cuando está configurada.
func (c *Client) allowed(number string) bool {
	if len(c.allowlist) == 0 {
		return true
	}
	return c.allowlist[onlyDigits(number)]
}

// ErrNotAllowed se devuelve cuando WHATSAPP_ALLOWLIST está activa y el número no está en la lista.
var ErrNotAllowed = errors.New("número no autorizado para esta demo")

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
	if !c.allowed(number) {
		return ErrNotAllowed
	}
	_, err := c.do("POST", "/message/sendText/"+c.instance, map[string]any{
		"number": number, "text": text,
	})
	return err
}

// SendDocument manda un archivo (ej. PDF) como adjunto — POST /message/sendMedia/{instance}
// con el contenido en base64 (sin el prefijo "data:...;base64,").
func (c *Client) SendDocument(number, fileName, mimeType, caption string, data []byte) error {
	if !c.allowed(number) {
		return ErrNotAllowed
	}
	_, err := c.do("POST", "/message/sendMedia/"+c.instance, map[string]any{
		"number":    number,
		"mediatype": "document",
		"mimetype":  mimeType,
		"fileName":  fileName,
		"caption":   caption,
		"media":     base64.StdEncoding.EncodeToString(data),
	})
	return err
}

// MediaResult es lo que devuelve Evolution al descargar un adjunto de un mensaje entrante.
type MediaResult struct {
	Base64   string `json:"base64"`
	MimeType string `json:"mimetype"`
	FileName string `json:"fileName"`
}

// GetMediaBase64 descarga el contenido (imagen/audio/documento) de un mensaje entrante —
// POST /chat/getBase64FromMediaMessage/{instance}, body {"message": <WAMessage crudo del
// webhook>}. rawMessage es el objeto "data" tal cual llega en el webhook de messages.upsert
// (mismo shape que espera Evolution: {key, message, ...}).
func (c *Client) GetMediaBase64(rawMessage json.RawMessage) (*MediaResult, error) {
	req, err := http.NewRequest("POST", c.baseURL+"/chat/getBase64FromMediaMessage/"+c.instance,
		bytes.NewReader(mustWrapMessage(rawMessage)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 30<<20)) // hasta 30MB de base64
	if resp.StatusCode >= 300 {
		return nil, &ErrorHTTP{Status: resp.StatusCode, Body: string(raw)}
	}
	var out MediaResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Base64 == "" {
		return nil, errors.New("evolution: media sin base64")
	}
	return &out, nil
}

func mustWrapMessage(rawMessage json.RawMessage) []byte {
	b, _ := json.Marshal(map[string]json.RawMessage{"message": rawMessage})
	return b
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

// Group es un grupo de WhatsApp al que pertenece la instancia (para elegir destino de mensajes).
type Group struct {
	JID     string `json:"id"`
	Subject string `json:"subject"`
	Size    int    `json:"size"`
}

// FetchGroups lista los grupos de la instancia. Evolution devuelve un array
// (no un objeto), por eso no reutiliza do().
func (c *Client) FetchGroups() ([]Group, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/group/fetchAllGroups/"+c.instance+"?getParticipants=false", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, &ErrorHTTP{Status: resp.StatusCode, Body: string(raw)}
	}
	var groups []Group
	if err := json.Unmarshal(raw, &groups); err != nil {
		return nil, err
	}
	return groups, nil
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
