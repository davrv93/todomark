package hydra

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client habla con la API admin de Ory Hydra (login/consent provider).
type Client struct {
	adminURL string
	http     *http.Client
}

func NewFromEnv() *Client {
	return &Client{
		adminURL: envOr("HYDRA_ADMIN_URL", "http://hydra:4445"),
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type acceptResponse struct {
	RedirectTo string `json:"redirect_to"`
}

func (c *Client) do(method, path string, body any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.adminURL+path, rd)
	if err != nil {
		return nil, err
	}
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
		return nil, fmt.Errorf("hydra admin status %d: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}

// AcceptLogin marca el login_challenge como autenticado por `subject`, con el
// rol otorgado (se propaga como scope, para que el introspect del gateway lo lea).
func (c *Client) AcceptLogin(challenge, subject string) (string, error) {
	raw, err := c.do(http.MethodPut, "/admin/oauth2/auth/requests/login/accept?login_challenge="+challenge, map[string]any{
		"subject":      subject,
		"remember":     true,
		"remember_for": 3600,
	})
	if err != nil {
		return "", err
	}
	var out acceptResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	return out.RedirectTo, nil
}

type consentRequest struct {
	RequestedScope []string `json:"requested_scope"`
	Subject        string   `json:"subject"`
}

// AcceptConsent otorga exactamente el scope solicitado por el cliente (primera
// parte, confiable: es nuestro propio SPA) y lo recuerda para no repreguntar.
func (c *Client) AcceptConsent(challenge string) (string, error) {
	raw, err := c.do(http.MethodGet, "/admin/oauth2/auth/requests/consent?consent_challenge="+challenge, nil)
	if err != nil {
		return "", err
	}
	var reqInfo consentRequest
	if err := json.Unmarshal(raw, &reqInfo); err != nil {
		return "", err
	}
	acceptRaw, err := c.do(http.MethodPut, "/admin/oauth2/auth/requests/consent/accept?consent_challenge="+challenge, map[string]any{
		"grant_scope":                 reqInfo.RequestedScope,
		"grant_access_token_audience": []string{},
		"remember":                    true,
		"remember_for":                3600,
	})
	if err != nil {
		return "", err
	}
	var out acceptResponse
	if err := json.Unmarshal(acceptRaw, &out); err != nil {
		return "", err
	}
	return out.RedirectTo, nil
}
