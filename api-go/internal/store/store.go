package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Event struct {
	Timestamp string `json:"timestamp"`
	User      string `json:"user"`
	Action    string `json:"action"`
	From      string `json:"from"`
	To        string `json:"to"`
}

type Ticket struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Status         string  `json:"status"`
	Priority       string  `json:"priority"`
	Requester      string  `json:"requester"`
	AssignedTo     *string `json:"assignedTo"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
	History        []Event `json:"history"`
	WhatsappChatID *string `json:"whatsappChatId"`
}

var ErrNotFound = errors.New("not found")

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite: una sola conexión evita SQLITE_BUSY en escrituras concurrentes
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS tickets (
	id              TEXT PRIMARY KEY,
	title           TEXT NOT NULL,
	description     TEXT NOT NULL DEFAULT '',
	status          TEXT NOT NULL DEFAULT 'open',
	priority        TEXT NOT NULL DEFAULT 'low',
	requester       TEXT NOT NULL DEFAULT '',
	assigned_to     TEXT,
	created_at      TEXT NOT NULL,
	updated_at      TEXT NOT NULL,
	history         TEXT NOT NULL DEFAULT '[]',
	whatsapp_chat_id TEXT
)`)
	return err
}

func nowISO() string { return time.Now().UTC().Format(time.RFC3339) }

func NewTicket(title, description, priority, requester string, assignedTo *string) *Ticket {
	now := nowISO()
	return &Ticket{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		Title:       title,
		Description: description,
		Status:      "open",
		Priority:    priority,
		Requester:   requester,
		AssignedTo:  assignedTo,
		CreatedAt:   now,
		UpdatedAt:   now,
		History:     []Event{},
	}
}

const ticketCols = "id,title,description,status,priority,requester,assigned_to,created_at,updated_at,history,whatsapp_chat_id"

func scanTicket(row interface{ Scan(...any) error }) (Ticket, error) {
	var t Ticket
	var hist string
	var assigned, chatID sql.NullString
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.Requester,
		&assigned, &t.CreatedAt, &t.UpdatedAt, &hist, &chatID)
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, err
	}
	if assigned.Valid {
		v := assigned.String
		t.AssignedTo = &v
	}
	if chatID.Valid {
		v := chatID.String
		t.WhatsappChatID = &v
	}
	t.History = []Event{}
	if hist != "" {
		json.Unmarshal([]byte(hist), &t.History)
	}
	return t, nil
}

func (s *Store) List(status, priority, query string) ([]Ticket, error) {
	q := "SELECT " + ticketCols + " FROM tickets WHERE 1=1"
	var args []any
	if status != "" {
		q += " AND status = ?"
		args = append(args, status)
	}
	if priority != "" {
		q += " AND priority = ?"
		args = append(args, priority)
	}
	if query != "" {
		q += " AND (title LIKE ? OR description LIKE ?)"
		like := "%" + query + "%"
		args = append(args, like, like)
	}
	q += " ORDER BY created_at DESC"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) Get(id string) (Ticket, error) {
	row := s.db.QueryRow("SELECT "+ticketCols+" FROM tickets WHERE id = ?", id)
	return scanTicket(row)
}

func (s *Store) Create(t *Ticket) error {
	hist, _ := json.Marshal(t.History)
	_, err := s.db.Exec(
		`INSERT INTO tickets (`+ticketCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Title, t.Description, t.Status, t.Priority, t.Requester,
		t.AssignedTo, t.CreatedAt, t.UpdatedAt, string(hist), t.WhatsappChatID,
	)
	return err
}

// Update carga el ticket, aplica mutate y persiste el resultado de forma atómica.
func (s *Store) Update(id string, mutate func(*Ticket) error) (Ticket, error) {
	t, err := s.Get(id)
	if err != nil {
		return t, err
	}
	if err := mutate(&t); err != nil {
		return t, err
	}
	t.UpdatedAt = nowISO()
	hist, _ := json.Marshal(t.History)
	_, err = s.db.Exec(
		`UPDATE tickets SET title=?, description=?, status=?, priority=?, requester=?,
		 assigned_to=?, updated_at=?, history=?, whatsapp_chat_id=? WHERE id=?`,
		t.Title, t.Description, t.Status, t.Priority, t.Requester,
		t.AssignedTo, t.UpdatedAt, string(hist), t.WhatsappChatID, t.ID,
	)
	if err != nil {
		return t, err
	}
	return t, nil
}

// ListByChat devuelve los tickets cuyo destinatario WhatsApp coincide (bot solo activo para ellos).
func (s *Store) ListByChat(chatID string) ([]Ticket, error) {
	rows, err := s.db.Query("SELECT "+ticketCols+" FROM tickets WHERE whatsapp_chat_id = ? ORDER BY updated_at DESC", chatID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) Delete(id string) error {
	res, err := s.db.Exec("DELETE FROM tickets WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
