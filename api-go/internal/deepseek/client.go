package deepseek

import (
	"bytes"
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

// systemPrompt es el prompt base definido en PLAN.md §20.5 para el chatbot de ventas.
const systemPrompt = "Actúa como asistente de ventas de Todo Mark ERP. Responde en español, tono profesional, sugiere demo o contacto. Respuestas ≤ 2 frases."

type Client struct {
	mu      sync.RWMutex
	apiKey  string
	baseURL string
	http    *http.Client
}

func NewFromEnv() *Client {
	return &Client{
		apiKey:  os.Getenv("DEEPSEEK_API_KEY"),
		baseURL: envOr("DEEPSEEK_API_URL", "https://api.deepseek.com/v1/chat/completions"),
		http:    &http.Client{Timeout: 20 * time.Second},
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

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func (c *Client) Chat(message string) (string, error) {
	apiKey := c.getAPIKey()
	if apiKey == "" {
		return "", errors.New("DEEPSEEK_API_KEY no configurado")
	}
	reqBody := chatRequest{
		Model: "deepseek-chat",
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: message},
		},
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("deepseek status %d: %s", resp.StatusCode, string(body))
	}
	var out chatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("deepseek: respuesta vacía")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
