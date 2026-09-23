package glpi

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

	"todomark/api/internal/store"
)

type Client struct {
	baseURL      string
	appToken     string
	user         string
	password     string
	sessionToken string
	http         *http.Client
}

type RemoteTicket struct {
	ID          string
	Title       string
	Description string
	Status      string
	Priority    string
	Requester   string
	AssignedTo  string
	CreatedAt   string
	UpdatedAt   string
	ClosedAt    string
}

func NewFromEnv() *Client {
	return &Client{
		baseURL:  strings.TrimRight(os.Getenv("GLPI_URL"), "/"),
		appToken: os.Getenv("GLPI_APP_TOKEN"),
		user:     os.Getenv("GLPI_USER"),
		password: os.Getenv("GLPI_PASSWORD"),
		http:     &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) Configured() bool {
	return c.baseURL != "" && c.appToken != "" && c.user != "" && c.password != ""
}

func (c *Client) InitSession() error {
	if !c.Configured() {
		return errors.New("GLPI no configurado")
	}
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/apirest.php/initSession", nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.user, c.password)
	req.Header.Set("App-Token", c.appToken)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("glpi initSession status %d: %s", resp.StatusCode, string(raw))
	}
	var out struct {
		SessionToken string `json:"session_token"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	if out.SessionToken == "" {
		return errors.New("GLPI no devolvio session_token")
	}
	c.sessionToken = out.SessionToken
	return nil
}

func (c *Client) KillSession() error {
	if c.sessionToken == "" {
		return nil
	}
	_, err := c.do(http.MethodGet, "/apirest.php/killSession", nil)
	c.sessionToken = ""
	return err
}

func (c *Client) ListTickets() ([]RemoteTicket, error) {
	data, err := c.do(http.MethodGet, "/apirest.php/Ticket", nil)
	if err != nil {
		return nil, err
	}
	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := []RemoteTicket{}
	for _, item := range raw {
		out = append(out, fromGLPI(item))
	}
	return out, nil
}

func (c *Client) CreateTicket(t store.Ticket) (RemoteTicket, error) {
	data, err := c.do(http.MethodPost, "/apirest.php/Ticket", map[string]any{"input": toGLPI(t)})
	if err != nil {
		return RemoteTicket{}, err
	}
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	return fromGLPI(raw), nil
}

func (c *Client) UpdateTicket(remoteID string, t store.Ticket) error {
	_, err := c.do(http.MethodPut, "/apirest.php/Ticket/"+remoteID, map[string]any{"input": toGLPI(t)})
	return err
}

func (c *Client) do(method, path string, body any) ([]byte, error) {
	if c.sessionToken == "" && !strings.Contains(path, "initSession") {
		if err := c.InitSession(); err != nil {
			return nil, err
		}
	}
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.baseURL+path, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("App-Token", c.appToken)
	req.Header.Set("Session-Token", c.sessionToken)
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
		return nil, fmt.Errorf("glpi status %d: %s", resp.StatusCode, string(raw))
	}
	return raw, nil
}

func fromGLPI(item map[string]any) RemoteTicket {
	return RemoteTicket{
		ID:          asString(item["id"]),
		Title:       firstString(item, "name", "title"),
		Description: firstString(item, "content", "description"),
		Status:      normalizeStatus(asString(item["status"])),
		Priority:    normalizePriority(asString(item["priority"])),
		Requester:   firstString(item, "requester", "users_id_recipient"),
		AssignedTo:  firstString(item, "assigned_to", "assigned_team", "groups_id_assign"),
		CreatedAt:   firstString(item, "date_creation", "created_at"),
		UpdatedAt:   firstString(item, "date_mod", "updated_at"),
		ClosedAt:    firstString(item, "closed_at", "solvedate", "closedate"),
	}
}

func toGLPI(t store.Ticket) map[string]any {
	input := map[string]any{
		"name":     t.Title,
		"content":  t.Description,
		"status":   t.Status,
		"priority": t.Priority,
	}
	if t.Email != nil {
		input["requester"] = *t.Email
	} else {
		input["requester"] = t.Requester
	}
	if t.AssignedTeam != nil {
		input["assigned_to"] = *t.AssignedTeam
	} else if t.AssignedTo != nil {
		input["assigned_to"] = *t.AssignedTo
	}
	if t.ClosedAt != nil {
		input["closed_at"] = *t.ClosedAt
	}
	return input
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := asString(m[key]); s != "" {
			return s
		}
	}
	return ""
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	case int:
		return fmt.Sprintf("%d", t)
	default:
		return ""
	}
}

func normalizeStatus(s string) string {
	switch strings.ToLower(s) {
	case "2", "in_progress", "processing":
		return "in_progress"
	case "5", "resolved", "solved":
		return "resolved"
	case "6", "closed":
		return "closed"
	default:
		return "open"
	}
}

func normalizePriority(s string) string {
	switch strings.ToLower(s) {
	case "4", "high":
		return "high"
	case "5", "6", "critical":
		return "critical"
	case "2", "medium":
		return "medium"
	default:
		return "low"
	}
}
