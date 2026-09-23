package gemini

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
	"sync"
	"time"
)

// systemPrompt es el prompt base para el chatbot de ventas (mismo rol que en internal/deepseek).
const systemPrompt = "Actúa como asistente de ventas de Todo Mark ERP. Responde en español, tono profesional, sugiere demo o contacto. Respuestas ≤ 2 frases."

type Client struct {
	mu          sync.RWMutex
	apiKey      string
	baseURL     string
	fallbackURL string
	http        *http.Client
}

func NewFromEnv() *Client {
	return &Client{
		apiKey: os.Getenv("GEMINI_API_KEY"),
		// gemini-2.0-flash-lite fue retirado por Google (404 "no longer available",
		// detectado 2026-09-23) — reemplazado por 3.5-flash-lite, que es lo que la
		// propia API sugiere en el error. Si Google lo vuelve a rotar, se overridea
		// con GEMINI_API_URL sin tocar código.
		baseURL:     envOr("GEMINI_API_URL", "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.5-flash-lite:generateContent"),
		fallbackURL: envOr("GEMINI_FALLBACK_API_URL", "https://generativelanguage.googleapis.com/v1beta/models/gemma-3-27b-it:generateContent"),
		// 90s y no 30s: transcribir una nota de voz sube el audio en base64 y tarda
		// más que un chat de texto — con 30s daba "context deadline exceeded".
		http:        &http.Client{Timeout: 90 * time.Second},
	}
}

// SetAPIKey actualiza la key en caliente (ej. guardada desde la UI de configuración),
// sin necesidad de reiniciar el proceso.
func (c *Client) SetAPIKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.apiKey = key
}

func (c *Client) getAPIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey
}

// Configured indica si hay API key configurada. Sin ella el chatbot queda
// wireado pero inerte: main.go responde el fallback sin llamar a Chat().
func (c *Client) Configured() bool {
	return c.getAPIKey() != ""
}

type inlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiPart struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inline_data,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type generateRequest struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
}

type generateResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
}

// ChatWithSystem es un alias de Chat con el nombre que usa reportchat (mismo shape que
// deepseek.Client.ChatWithSystem, así main.go puede tratar ambos clientes por interfaz).
func (c *Client) ChatWithSystem(sysPrompt, message string) (string, error) {
	return c.Chat(sysPrompt, message)
}

// Chat envía systemPrompt (rol) + message (turno del usuario) a Gemini y devuelve el texto de respuesta.
func (c *Client) Chat(sysPrompt, message string) (string, error) {
	reqBody := generateRequest{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: sysPrompt}}},
		Contents:          []geminiContent{{Parts: []geminiPart{{Text: message}}}},
	}
	return c.generate(c.baseURL, reqBody)
}

// Transcribe manda audio (nota de voz de WhatsApp, ej. audio/ogg) a Gemini para transcripción.
// Intenta primero con el modelo principal (baseURL, flash-lite por default); si esa llamada
// falla por cuota (HTTP 429), reintenta con fallbackURL (gemma por default) — a pedido del
// usuario 2026-09-23, para no depender de un solo modelo/cuota.
func (c *Client) Transcribe(audioBytes []byte, mimeType string) (string, error) {
	reqBody := generateRequest{
		Contents: []geminiContent{{Parts: []geminiPart{
			{Text: "Transcribe este audio a texto en español. Responde solo con la transcripción, sin comentarios adicionales ni comillas."},
			{InlineData: &inlineData{MimeType: mimeType, Data: base64.StdEncoding.EncodeToString(audioBytes)}},
		}}},
	}
	text, err := c.generate(c.baseURL, reqBody)
	if err != nil && isQuotaError(err) && c.fallbackURL != "" {
		text, err = c.generate(c.fallbackURL, reqBody)
	}
	return text, err
}

// ChatWithMedia es Chat + un adjunto (imagen o audio) en la misma consulta — entrada
// multimodal del bot de WhatsApp. Mismo camino que Chat/Transcribe, solo cambia el part.
func (c *Client) ChatWithMedia(sysPrompt, message, mimeType string, data []byte) (string, error) {
	reqBody := generateRequest{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: sysPrompt}}},
		Contents: []geminiContent{{Parts: []geminiPart{
			{Text: message},
			{InlineData: &inlineData{MimeType: mimeType, Data: base64.StdEncoding.EncodeToString(data)}},
		}}},
	}
	return c.generate(c.baseURL, reqBody)
}

func isQuotaError(err error) bool {
	return strings.Contains(err.Error(), "status 429")
}

// IsQuotaError expone isQuotaError a quien llama desde afuera del paquete (ej. el bot de
// WhatsApp, que avisa al usuario y reintenta una sola vez).
func IsQuotaError(err error) bool {
	return err != nil && isQuotaError(err)
}

func (c *Client) generate(url string, reqBody generateRequest) (string, error) {
	apiKey := c.getAPIKey()
	if apiKey == "" {
		return "", errors.New("GEMINI_API_KEY no configurado")
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-goog-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("gemini status %d: %s", resp.StatusCode, string(body))
	}
	var out generateResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("gemini: respuesta vacía")
	}
	return strings.TrimSpace(out.Candidates[0].Content.Parts[0].Text), nil
}

// DefaultSystemPrompt expone el prompt base para que main.go lo reutilice al armar
// el prompt final (ej. con contexto de RAG inyectado).
func DefaultSystemPrompt() string { return systemPrompt }

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
