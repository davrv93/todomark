package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	ID                string  `json:"id"`
	Title             string  `json:"title"`
	Description       string  `json:"description"`
	Status            string  `json:"status"`
	Priority          string  `json:"priority"`
	Requester         string  `json:"requester"`
	AssignedTo        *string `json:"assignedTo"`
	Email             *string `json:"email"`
	AssignedTeam      *string `json:"assignedTeam"`
	ClosedAt          *string `json:"closedAt"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
	History           []Event `json:"history"`
	WhatsappChatID    *string `json:"whatsappChatId"`
	RemoteID          *string `json:"remoteId"`
	LastSyncAt        *string `json:"lastSyncAt"`
	RemoteUpdatedAt   *string `json:"remoteUpdatedAt"`
	Channel           *string `json:"channel"`
	SatisfactionScore *int    `json:"satisfactionScore"`
	Category          *string `json:"category"`
	BuildingID        *string `json:"buildingId"`
	UnitID            *string `json:"unitId"`
}

type Building struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Tier string `json:"tier"`
}

type WeeklyBucket struct {
	WeekStart string `json:"weekStart"`
	Created   int    `json:"created"`
	Closed    int    `json:"closed"`
}

type BuildingStat struct {
	BuildingID         string   `json:"buildingId"`
	Name               string   `json:"name"`
	Tier               string   `json:"tier"`
	Count              int      `json:"count"`
	AvgResolutionHours *float64 `json:"avgResolutionHours"`
}

type CategoryByBuilding struct {
	BuildingID   string `json:"buildingId"`
	BuildingName string `json:"buildingName"`
	Category     string `json:"category"`
	Count        int    `json:"count"`
}

type ReportSummary struct {
	ByStatus               map[string]int `json:"byStatus"`
	ByPriority             map[string]int `json:"byPriority"`
	ByChannel              map[string]int `json:"byChannel"`
	ByAgent                map[string]int `json:"byAgent"`
	ByCategory             map[string]int `json:"byCategory"`
	DailyTickets           map[string]int `json:"dailyTickets"`
	AverageResolutionHours *float64       `json:"averageResolutionHours"`
	// Operativos (O1/O3/O4/O5/O6/O8 de DOCUMENTACION.md §7.2)
	AgingHoursByStatus    map[string]float64 `json:"agingHoursByStatus"`
	UnassignedCount       int                `json:"unassignedCount"`
	UnassignedOldestHours *float64           `json:"unassignedOldestHours"`
	ReopensToday          int                `json:"reopensToday"`
	SLAAtRiskCount        int                `json:"slaAtRiskCount"`
	SLAOverdueCount       int                `json:"slaOverdueCount"`
	FRTHours              *float64           `json:"frtHours"`
	WeeklyBacklog         []WeeklyBucket     `json:"weeklyBacklog"`
	// Gerencial avanzado (G6/G7) — building_id/tier vienen de datos DEMO, ver DOCUMENTACION.md §13.
	ByBuilding         []BuildingStat       `json:"byBuilding"`
	CategoryByBuilding []CategoryByBuilding `json:"categoryByBuilding"`
}

type ExecutiveSummary struct {
	OpenNow          int            `json:"openNow"`
	NetChange7d      int            `json:"netChange7d"`
	MTTRHours        *float64       `json:"mttrHours"`
	ReopenRatePct    *float64       `json:"reopenRatePct"`
	CSATAvg          *float64       `json:"csatAvg"`
	SLACompliancePct *float64       `json:"slaCompliancePct"`
	WeeklyTrend      []WeeklyBucket `json:"weeklyTrend"`
	CostEstimateDemo *float64       `json:"costEstimateDemo"`
	LandingVisits    int            `json:"landingVisits"`
	LeadsTotal       int            `json:"leadsTotal"`
}

// slaTargetHoursDemo son umbrales propuestos, NO confirmados por un gerente real — sirven para la
// demo hasta que F0 (REGLAS/DOCUMENTACION.md §13-14) defina el umbral de negocio real por prioridad.
var slaTargetHoursDemo = map[string]float64{
	"critical": 4,
	"high":     24,
	"medium":   72,
	"low":      168,
}

// costoHoraDemo (USD) es una tarifa FICTICIA para estimar costo de soporte (G8) en este proyecto
// demostrativo — autorizado explícitamente por el usuario 2026-09-23. NO es la tarifa real de
// ningún cliente. Reemplazar por un valor configurable real antes de cualquier entrega real.
const costoHoraDemo = 25.0

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
	if _, err := db.Exec(`
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
)`); err != nil {
		return err
	}
	for _, col := range []struct {
		name string
		ddl  string
	}{
		{"email", "ALTER TABLE tickets ADD COLUMN email TEXT"},
		{"assigned_team", "ALTER TABLE tickets ADD COLUMN assigned_team TEXT"},
		{"closed_at", "ALTER TABLE tickets ADD COLUMN closed_at DATETIME"},
		{"remote_id", "ALTER TABLE tickets ADD COLUMN remote_id TEXT"},
		{"last_sync_at", "ALTER TABLE tickets ADD COLUMN last_sync_at TEXT"},
		{"remote_updated_at", "ALTER TABLE tickets ADD COLUMN remote_updated_at TEXT"},
		{"channel", "ALTER TABLE tickets ADD COLUMN channel TEXT"},
		{"satisfaction_score", "ALTER TABLE tickets ADD COLUMN satisfaction_score INTEGER"},
		{"category", "ALTER TABLE tickets ADD COLUMN category TEXT"},
	} {
		if err := ensureColumn(db, "tickets", col.name, col.ddl); err != nil {
			return err
		}
	}
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS processed_events (
	event_id TEXT PRIMARY KEY,
	processed_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS glpi_conflicts (
	id TEXT PRIMARY KEY,
	ticket_id TEXT NOT NULL,
	remote_id TEXT NOT NULL,
	local_updated_at TEXT NOT NULL,
	remote_updated_at TEXT NOT NULL,
	detected_at TEXT NOT NULL,
	details TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS whatsapp_messages (
	id TEXT PRIMARY KEY,
	chat_id TEXT NOT NULL,
	chat_name TEXT NOT NULL DEFAULT '',
	direction TEXT NOT NULL CHECK(direction IN ('in','out')),
	text TEXT NOT NULL DEFAULT '',
	ticket_id TEXT,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_chat ON whatsapp_messages(chat_id, created_at);
CREATE TABLE IF NOT EXISTS attachments (
	id TEXT PRIMARY KEY,
	ticket_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	mime_type TEXT NOT NULL DEFAULT '',
	file_name TEXT NOT NULL DEFAULT '',
	file_path TEXT NOT NULL,
	transcript TEXT,
	source TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_attachments_ticket ON attachments(ticket_id, created_at);
CREATE TABLE IF NOT EXISTS leads (
	id TEXT PRIMARY KEY,
	name TEXT,
	email TEXT,
	company TEXT,
	created_at TEXT
);
CREATE TABLE IF NOT EXISTS settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS knowledge_chunks (
	id TEXT PRIMARY KEY,
	source TEXT NOT NULL,
	text TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS buildings (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	tier TEXT NOT NULL DEFAULT 'standard'
);
CREATE TABLE IF NOT EXISTS landing_visits (
	day TEXT PRIMARY KEY,
	count INTEGER NOT NULL DEFAULT 0
)`); err != nil {
		return err
	}
	for _, col := range []struct {
		name string
		ddl  string
	}{
		{"building_id", "ALTER TABLE tickets ADD COLUMN building_id TEXT"},
		{"unit_id", "ALTER TABLE tickets ADD COLUMN unit_id TEXT"},
	} {
		if err := ensureColumn(db, "tickets", col.name, col.ddl); err != nil {
			return err
		}
	}
	// Edificios demo — DATOS FICTICIOS para poblar reportes G6/G7/U2/U3 en este proyecto
	// demostrativo (autorizado explícitamente por el usuario 2026-09-23, ver DOCUMENTACION.md §13).
	// NUNCA presentar estos 3 nombres como edificios reales de un cliente real.
	for _, b := range []struct{ id, name, tier string }{
		{"bld_torre_norte", "Torre Norte (demo)", "vip"},
		{"bld_edificio_sur", "Edificio Sur (demo)", "standard"},
		{"bld_res_central", "Residencial Central (demo)", "standard"},
	} {
		if _, err := db.Exec("INSERT OR IGNORE INTO buildings (id, name, tier) VALUES (?, ?, ?)", b.id, b.name, b.tier); err != nil {
			return err
		}
	}
	return nil
}

func ensureColumn(db *sql.DB, table, name, ddl string) error {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var colName, colType string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if colName == name {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(ddl)
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

const ticketCols = "id,title,description,status,priority,requester,assigned_to,email,assigned_team,closed_at,created_at,updated_at,history,whatsapp_chat_id,remote_id,last_sync_at,remote_updated_at,channel,satisfaction_score,category,building_id,unit_id"

func scanTicket(row interface{ Scan(...any) error }) (Ticket, error) {
	var t Ticket
	var hist string
	var assigned, email, team, closedAt, chatID, remoteID, lastSyncAt, remoteUpdatedAt, channel, category, buildingID, unitID sql.NullString
	var satisfaction sql.NullInt64
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.Priority, &t.Requester,
		&assigned, &email, &team, &closedAt, &t.CreatedAt, &t.UpdatedAt, &hist, &chatID, &remoteID, &lastSyncAt, &remoteUpdatedAt, &channel, &satisfaction, &category, &buildingID, &unitID)
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
	if email.Valid {
		v := email.String
		t.Email = &v
	}
	if team.Valid {
		v := team.String
		t.AssignedTeam = &v
	}
	if closedAt.Valid {
		v := closedAt.String
		t.ClosedAt = &v
	}
	if chatID.Valid {
		v := chatID.String
		t.WhatsappChatID = &v
	}
	if remoteID.Valid {
		v := remoteID.String
		t.RemoteID = &v
	}
	if lastSyncAt.Valid {
		v := lastSyncAt.String
		t.LastSyncAt = &v
	}
	if remoteUpdatedAt.Valid {
		v := remoteUpdatedAt.String
		t.RemoteUpdatedAt = &v
	}
	if channel.Valid {
		v := channel.String
		t.Channel = &v
	}
	if satisfaction.Valid {
		v := int(satisfaction.Int64)
		t.SatisfactionScore = &v
	}
	if category.Valid {
		v := category.String
		t.Category = &v
	}
	if buildingID.Valid {
		v := buildingID.String
		t.BuildingID = &v
	}
	if unitID.Valid {
		v := unitID.String
		t.UnitID = &v
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
		`INSERT INTO tickets (`+ticketCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.Title, t.Description, t.Status, t.Priority, t.Requester,
		t.AssignedTo, t.Email, t.AssignedTeam, t.ClosedAt, t.CreatedAt, t.UpdatedAt, string(hist), t.WhatsappChatID,
		t.RemoteID, t.LastSyncAt, t.RemoteUpdatedAt, t.Channel, t.SatisfactionScore, t.Category, t.BuildingID, t.UnitID,
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
		 assigned_to=?, email=?, assigned_team=?, closed_at=?, updated_at=?, history=?, whatsapp_chat_id=?,
		 remote_id=?, last_sync_at=?, remote_updated_at=? WHERE id=?`,
		t.Title, t.Description, t.Status, t.Priority, t.Requester,
		t.AssignedTo, t.Email, t.AssignedTeam, t.ClosedAt, t.UpdatedAt, string(hist), t.WhatsappChatID,
		t.RemoteID, t.LastSyncAt, t.RemoteUpdatedAt, t.ID,
	)
	if err != nil {
		return t, err
	}
	return t, nil
}

// SetSatisfaction registra el CSAT (1-5) que el cliente final da a un ticket ya resuelto/cerrado.
// No pasa por Update (su SET no incluye esta columna, igual que remote_id) para no tocar ese camino.
func (s *Store) SetSatisfaction(id string, score int) (Ticket, error) {
	res, err := s.db.Exec("UPDATE tickets SET satisfaction_score = ? WHERE id = ?", score, id)
	if err != nil {
		return Ticket{}, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return Ticket{}, err
	} else if n == 0 {
		return Ticket{}, ErrNotFound
	}
	return s.Get(id)
}

func (s *Store) MarkProcessed(eventID string) (bool, error) {
	if eventID == "" {
		eventID = fmt.Sprintf("generated-%d", time.Now().UnixNano())
	}
	res, err := s.db.Exec("INSERT OR IGNORE INTO processed_events(event_id, processed_at) VALUES (?, ?)", eventID, nowISO())
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

func (s *Store) FindByRemoteID(remoteID string) (Ticket, error) {
	row := s.db.QueryRow("SELECT "+ticketCols+" FROM tickets WHERE remote_id = ?", remoteID)
	return scanTicket(row)
}

func (s *Store) All() ([]Ticket, error) {
	return s.List("", "", "")
}

func (s *Store) RecordGLPIConflict(ticketID, remoteID, localUpdatedAt, remoteUpdatedAt, details string) error {
	id := fmt.Sprintf("%s-%s", ticketID, remoteID)
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO glpi_conflicts(id,ticket_id,remote_id,local_updated_at,remote_updated_at,detected_at,details)
		 VALUES (?,?,?,?,?,?,?)`,
		id, ticketID, remoteID, localUpdatedAt, remoteUpdatedAt, nowISO(), details,
	)
	return err
}

func (s *Store) ListByEmail(email string) ([]Ticket, error) {
	return s.listBy("email", email)
}

func (s *Store) ListByTeam(team string) ([]Ticket, error) {
	return s.listBy("assigned_team", team)
}

func (s *Store) listBy(column, value string) ([]Ticket, error) {
	rows, err := s.db.Query("SELECT "+ticketCols+" FROM tickets WHERE "+column+" = ? ORDER BY updated_at DESC", value)
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

func (s *Store) ReportSummary() (ReportSummary, error) {
	summary := ReportSummary{
		ByStatus:     map[string]int{},
		ByPriority:   map[string]int{},
		ByChannel:    map[string]int{},
		ByAgent:      map[string]int{},
		DailyTickets: map[string]int{},
	}
	if err := countGrouped(s.db, "status", summary.ByStatus); err != nil {
		return summary, err
	}
	if err := countGrouped(s.db, "priority", summary.ByPriority); err != nil {
		return summary, err
	}
	// channel es nullable (tickets creados antes de esta migración no lo tienen) — COALESCE
	// a "sin_canal" porque Scan de NULL en *string falla.
	channelRows, err := s.db.Query("SELECT COALESCE(channel, 'sin_canal'), COUNT(*) FROM tickets GROUP BY COALESCE(channel, 'sin_canal')")
	if err != nil {
		return summary, err
	}
	for channelRows.Next() {
		var key string
		var count int
		if err := channelRows.Scan(&key, &count); err != nil {
			channelRows.Close()
			return summary, err
		}
		summary.ByChannel[key] = count
	}
	if err := channelRows.Close(); err != nil {
		return summary, err
	}
	// assigned_to también es nullable — mismo tratamiento que channel (O2: carga por agente).
	agentRows, err := s.db.Query("SELECT COALESCE(assigned_to, 'sin_asignar'), COUNT(*) FROM tickets WHERE status IN ('open','in_progress','reopened') GROUP BY COALESCE(assigned_to, 'sin_asignar')")
	if err != nil {
		return summary, err
	}
	for agentRows.Next() {
		var key string
		var count int
		if err := agentRows.Scan(&key, &count); err != nil {
			agentRows.Close()
			return summary, err
		}
		summary.ByAgent[key] = count
	}
	if err := agentRows.Close(); err != nil {
		return summary, err
	}
	// category es nullable — mismo tratamiento que channel (U4: historial por categoría).
	catRows, err := s.db.Query("SELECT COALESCE(category, 'sin_categoria'), COUNT(*) FROM tickets GROUP BY COALESCE(category, 'sin_categoria')")
	if err != nil {
		return summary, err
	}
	summary.ByCategory = map[string]int{}
	for catRows.Next() {
		var key string
		var count int
		if err := catRows.Scan(&key, &count); err != nil {
			catRows.Close()
			return summary, err
		}
		summary.ByCategory[key] = count
	}
	if err := catRows.Close(); err != nil {
		return summary, err
	}
	rows, err := s.db.Query("SELECT substr(created_at, 1, 10) AS day, COUNT(*) FROM tickets GROUP BY day ORDER BY day")
	if err != nil {
		return summary, err
	}
	for rows.Next() {
		var day string
		var count int
		if err := rows.Scan(&day, &count); err != nil {
			rows.Close()
			return summary, err
		}
		summary.DailyTickets[day] = count
	}
	if err := rows.Close(); err != nil {
		return summary, err
	}
	var avg sql.NullFloat64
	if err := s.db.QueryRow(`SELECT AVG((julianday(closed_at) - julianday(created_at)) * 24) FROM tickets WHERE closed_at IS NOT NULL`).Scan(&avg); err != nil {
		return summary, err
	}
	if avg.Valid {
		summary.AverageResolutionHours = &avg.Float64
	}

	// G6: ranking de edificios por volumen y MTTR. building_id/tier son DEMO (ver DOCUMENTACION.md §13).
	buildingRows, err := s.db.Query(`
SELECT COALESCE(t.building_id, 'sin_edificio'), COALESCE(b.name, 'Sin edificio'), COALESCE(b.tier, ''), COUNT(*),
       AVG(CASE WHEN t.closed_at IS NOT NULL THEN (julianday(t.closed_at) - julianday(t.created_at)) * 24 END)
FROM tickets t LEFT JOIN buildings b ON b.id = t.building_id
GROUP BY 1, 2, 3
ORDER BY COUNT(*) DESC`)
	if err != nil {
		return summary, err
	}
	for buildingRows.Next() {
		var bs BuildingStat
		var avg sql.NullFloat64
		if err := buildingRows.Scan(&bs.BuildingID, &bs.Name, &bs.Tier, &bs.Count, &avg); err != nil {
			buildingRows.Close()
			return summary, err
		}
		if avg.Valid {
			bs.AvgResolutionHours = &avg.Float64
		}
		summary.ByBuilding = append(summary.ByBuilding, bs)
	}
	if err := buildingRows.Close(); err != nil {
		return summary, err
	}

	// G7: heatmap edificio × categoría (tabla plana, el front la pivotea).
	cbRows, err := s.db.Query(`
SELECT COALESCE(t.building_id, 'sin_edificio'), COALESCE(b.name, 'Sin edificio'), COALESCE(t.category, 'sin_categoria'), COUNT(*)
FROM tickets t LEFT JOIN buildings b ON b.id = t.building_id
GROUP BY 1, 2, 3`)
	if err != nil {
		return summary, err
	}
	for cbRows.Next() {
		var cb CategoryByBuilding
		if err := cbRows.Scan(&cb.BuildingID, &cb.BuildingName, &cb.Category, &cb.Count); err != nil {
			cbRows.Close()
			return summary, err
		}
		summary.CategoryByBuilding = append(summary.CategoryByBuilding, cb)
	}
	if err := cbRows.Close(); err != nil {
		return summary, err
	}

	tickets, err := s.All()
	if err != nil {
		return summary, err
	}
	if err := fillOperationalReports(&summary, tickets); err != nil {
		return summary, err
	}
	return summary, nil
}

// fillOperationalReports calcula O1 (aging por estado), O3 (sin asignar), O4 (reaperturas hoy),
// O5 (SLA en riesgo/vencido, umbral demo), O6 (FRT) y O8 (backlog semanal) — todo desde
// timestamps reales de tickets/history, sin inventar datos de negocio.
func fillOperationalReports(summary *ReportSummary, tickets []Ticket) error {
	now := time.Now().UTC()
	today := now.Format("2006-01-02")

	agingSum := map[string]float64{}
	agingCount := map[string]int{}
	var unassignedOldest *time.Time
	var frtTotalHours float64
	var frtCount int

	for _, t := range tickets {
		created, cerr := time.Parse(time.RFC3339, t.CreatedAt)

		lastStatusAt := created
		lastStatusOK := cerr == nil
		for i := len(t.History) - 1; i >= 0; i-- {
			if t.History[i].To == t.Status {
				if ts, err := time.Parse(time.RFC3339, t.History[i].Timestamp); err == nil {
					lastStatusAt = ts
					lastStatusOK = true
				}
				break
			}
		}
		if lastStatusOK {
			agingSum[t.Status] += now.Sub(lastStatusAt).Hours()
			agingCount[t.Status]++
		}

		isOpenLike := t.Status == "open" || t.Status == "in_progress" || t.Status == "reopened"

		if isOpenLike && t.AssignedTo == nil {
			summary.UnassignedCount++
			if cerr == nil && (unassignedOldest == nil || created.Before(*unassignedOldest)) {
				unassignedOldest = &created
			}
		}

		for _, ev := range t.History {
			if ev.To == "reopened" {
				if ts, err := time.Parse(time.RFC3339, ev.Timestamp); err == nil && ts.Format("2006-01-02") == today {
					summary.ReopensToday++
				}
			}
		}

		if isOpenLike && cerr == nil {
			if target, ok := slaTargetHoursDemo[t.Priority]; ok {
				remaining := target - now.Sub(created).Hours()
				switch {
				case remaining < 0:
					summary.SLAOverdueCount++
				case remaining <= 2:
					summary.SLAAtRiskCount++
				}
			}
		}

		if cerr == nil {
			for _, ev := range t.History {
				if ev.Action != "status_change" {
					continue
				}
				if ts, err := time.Parse(time.RFC3339, ev.Timestamp); err == nil {
					frtTotalHours += ts.Sub(created).Hours()
					frtCount++
				}
				break
			}
		}
	}

	summary.AgingHoursByStatus = map[string]float64{}
	for status, sum := range agingSum {
		summary.AgingHoursByStatus[status] = sum / float64(agingCount[status])
	}
	if unassignedOldest != nil {
		hours := now.Sub(*unassignedOldest).Hours()
		summary.UnassignedOldestHours = &hours
	}
	if frtCount > 0 {
		avg := frtTotalHours / float64(frtCount)
		summary.FRTHours = &avg
	}
	summary.WeeklyBacklog = weeklySeries(tickets, 8)
	return nil
}

// weeklySeries agrupa tickets por semana (lunes de inicio) para las últimas `weeks` semanas —
// usado por O8 (backlog semanal) y G5 (tendencia de volumen), misma data, dos audiencias.
func weeklySeries(tickets []Ticket, weeks int) []WeeklyBucket {
	now := time.Now().UTC()
	order := make([]string, 0, weeks)
	buckets := map[string]*WeeklyBucket{}
	for i := weeks - 1; i >= 0; i-- {
		wk := weekStart(now.AddDate(0, 0, -7*i))
		buckets[wk] = &WeeklyBucket{WeekStart: wk}
		order = append(order, wk)
	}
	earliest, _ := time.Parse("2006-01-02", order[0])
	for _, t := range tickets {
		if created, err := time.Parse(time.RFC3339, t.CreatedAt); err == nil && !created.Before(earliest) {
			if b, ok := buckets[weekStart(created)]; ok {
				b.Created++
			}
		}
		if t.ClosedAt != nil {
			if closed, err := time.Parse(time.RFC3339, *t.ClosedAt); err == nil && !closed.Before(earliest) {
				if b, ok := buckets[weekStart(closed)]; ok {
					b.Closed++
				}
			}
		}
	}
	out := make([]WeeklyBucket, 0, len(order))
	for _, wk := range order {
		out = append(out, *buckets[wk])
	}
	return out
}

func weekStart(t time.Time) string {
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	monday := t.AddDate(0, 0, -(wd - 1))
	return monday.Format("2006-01-02")
}

// ExecutiveSummary calcula KPIs de una sola pantalla para el dashboard gerencial
// (G1/G2/G3/G4/G10 de DOCUMENTACION.md §7.2). SLACompliancePct usa umbrales demo
// (slaTargetHoursDemo), NO confirmados por un gerente real — ver DOCUMENTACION.md §13-14.
func (s *Store) ExecutiveSummary() (ExecutiveSummary, error) {
	tickets, err := s.All()
	if err != nil {
		return ExecutiveSummary{}, err
	}
	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)

	var out ExecutiveSummary
	var mttrTotalHours float64
	var mttrCount int
	var reopenedTickets int
	var everClosedOrResolved int
	var created7d, closed7d int
	var slaTotal, slaMet int

	for _, t := range tickets {
		switch t.Status {
		case "open", "in_progress", "reopened":
			out.OpenNow++
		}
		if t.Status == "resolved" || t.Status == "closed" || t.Status == "reopened" {
			everClosedOrResolved++
		}
		for _, ev := range t.History {
			if ev.To == "reopened" {
				reopenedTickets++
				break
			}
		}
		if created, err := time.Parse(time.RFC3339, t.CreatedAt); err == nil && created.After(cutoff) {
			created7d++
		}
		if t.ClosedAt != nil {
			if closed, err := time.Parse(time.RFC3339, *t.ClosedAt); err == nil {
				if closed.After(cutoff) {
					closed7d++
				}
				if created, err := time.Parse(time.RFC3339, t.CreatedAt); err == nil {
					resolutionHours := closed.Sub(created).Hours()
					mttrTotalHours += resolutionHours
					mttrCount++
					if target, ok := slaTargetHoursDemo[t.Priority]; ok {
						slaTotal++
						if resolutionHours <= target {
							slaMet++
						}
					}
				}
			}
		}
	}
	out.NetChange7d = created7d - closed7d
	if mttrCount > 0 {
		avg := mttrTotalHours / float64(mttrCount)
		out.MTTRHours = &avg
	}
	if everClosedOrResolved > 0 {
		pct := 100 * float64(reopenedTickets) / float64(everClosedOrResolved)
		out.ReopenRatePct = &pct
	}
	if slaTotal > 0 {
		pct := 100 * float64(slaMet) / float64(slaTotal)
		out.SLACompliancePct = &pct
	}
	var csat sql.NullFloat64
	if err := s.db.QueryRow("SELECT AVG(satisfaction_score) FROM tickets WHERE satisfaction_score IS NOT NULL").Scan(&csat); err != nil {
		return out, err
	}
	if csat.Valid {
		out.CSATAvg = &csat.Float64
	}
	out.WeeklyTrend = weeklySeries(tickets, 8)

	if mttrCount > 0 {
		cost := mttrTotalHours * costoHoraDemo
		out.CostEstimateDemo = &cost
	}
	visits, err := s.LandingVisitsTotal()
	if err != nil {
		return out, err
	}
	out.LandingVisits = visits
	leads, err := s.CountLeads()
	if err != nil {
		return out, err
	}
	out.LeadsTotal = leads

	return out, nil
}

func countGrouped(db *sql.DB, column string, out map[string]int) error {
	rows, err := db.Query("SELECT " + column + ", COUNT(*) FROM tickets GROUP BY " + column)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var count int
		if err := rows.Scan(&key, &count); err != nil {
			return err
		}
		out[key] = count
	}
	return rows.Err()
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

type Message struct {
	ID        string  `json:"id"`
	ChatID    string  `json:"chatId"`
	ChatName  string  `json:"chatName"`
	Direction string  `json:"direction"`
	Text      string  `json:"text"`
	TicketID  *string `json:"ticketId"`
	CreatedAt string  `json:"createdAt"`
}

type Thread struct {
	ChatID       string `json:"chatId"`
	ChatName     string `json:"chatName"`
	LastText     string `json:"lastText"`
	LastAt       string `json:"lastAt"`
	MessageCount int    `json:"messageCount"`
}

func (s *Store) SaveMessage(m *Message) error {
	m.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	m.CreatedAt = nowISO()
	_, err := s.db.Exec(
		"INSERT INTO whatsapp_messages(id,chat_id,chat_name,direction,text,ticket_id,created_at) VALUES (?,?,?,?,?,?,?)",
		m.ID, m.ChatID, m.ChatName, m.Direction, m.Text, m.TicketID, m.CreatedAt,
	)
	return err
}

// ListThreads devuelve como mucho `limit` hilos (por defecto/tope 50/200),
// ordenados por último mensaje. `before` (cursor = last_at de la última página
// leída) trae la página siguiente, más vieja.
func (s *Store) ListThreads(limit int, before string) ([]Thread, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	having := ""
	args := []any{}
	if before != "" {
		having = "HAVING last_at < ?"
		args = append(args, before)
	}
	args = append(args, limit)
	rows, err := s.db.Query(fmt.Sprintf(`
SELECT chat_id, COALESCE(NULLIF(chat_name, ''), chat_id) as chat_name,
       (SELECT text FROM whatsapp_messages m2 WHERE m2.chat_id = m1.chat_id ORDER BY created_at DESC LIMIT 1) as last_text,
       MAX(created_at) as last_at, COUNT(*) as message_count
FROM whatsapp_messages m1
GROUP BY chat_id
%s
ORDER BY last_at DESC
LIMIT ?`, having), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Thread{}
	for rows.Next() {
		var t Thread
		if err := rows.Scan(&t.ChatID, &t.ChatName, &t.LastText, &t.LastAt, &t.MessageCount); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListMessages devuelve como mucho `limit` mensajes (por defecto/tope 100/500)
// de un chat, en orden cronológico ascendente. `before` (cursor = created_at
// del mensaje más viejo ya cargado) trae la página anterior (más vieja).
func (s *Store) ListMessages(chatID string, limit int, before string) ([]Message, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := "SELECT id,chat_id,chat_name,direction,text,ticket_id,created_at FROM whatsapp_messages WHERE chat_id = ?"
	args := []any{chatID}
	if before != "" {
		query += " AND created_at < ?"
		args = append(args, before)
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var ticketID sql.NullString
		if err := rows.Scan(&m.ID, &m.ChatID, &m.ChatName, &m.Direction, &m.Text, &ticketID, &m.CreatedAt); err != nil {
			return nil, err
		}
		if ticketID.Valid {
			v := ticketID.String
			m.TicketID = &v
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// SaveLead persiste un lead capturado desde el formulario de la landing.
// Solo guarda en SQLite: no hay integración con ningún CRM externo.
// Attachment: foto o nota de voz (con transcripción opcional) ligada a un ticket.
// "source" queda fijo en "whatsapp" hoy (único canal que adjunta archivos), se deja como
// campo libre por si en el futuro se suman adjuntos desde la web.
type Attachment struct {
	ID         string  `json:"id"`
	TicketID   string  `json:"ticketId"`
	Kind       string  `json:"kind"` // "photo" | "audio"
	MimeType   string  `json:"mimeType"`
	FileName   string  `json:"fileName"`
	FilePath   string  `json:"-"` // ruta en disco, no se expone por API (se sirve vía endpoint dedicado)
	Transcript *string `json:"transcript"`
	Source     string  `json:"source"`
	CreatedAt  string  `json:"createdAt"`
}

func (s *Store) SaveAttachment(a *Attachment) error {
	_, err := s.db.Exec(
		`INSERT INTO attachments(id,ticket_id,kind,mime_type,file_name,file_path,transcript,source,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		a.ID, a.TicketID, a.Kind, a.MimeType, a.FileName, a.FilePath, a.Transcript, a.Source, a.CreatedAt,
	)
	return err
}

func (s *Store) ListAttachments(ticketID string) ([]Attachment, error) {
	rows, err := s.db.Query(
		`SELECT id,ticket_id,kind,mime_type,file_name,file_path,transcript,source,created_at FROM attachments WHERE ticket_id = ? ORDER BY created_at`,
		ticketID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Attachment{}
	for rows.Next() {
		var a Attachment
		var transcript sql.NullString
		if err := rows.Scan(&a.ID, &a.TicketID, &a.Kind, &a.MimeType, &a.FileName, &a.FilePath, &transcript, &a.Source, &a.CreatedAt); err != nil {
			return nil, err
		}
		if transcript.Valid {
			a.Transcript = &transcript.String
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) GetAttachment(id string) (Attachment, error) {
	var a Attachment
	var transcript sql.NullString
	err := s.db.QueryRow(
		`SELECT id,ticket_id,kind,mime_type,file_name,file_path,transcript,source,created_at FROM attachments WHERE id = ?`, id,
	).Scan(&a.ID, &a.TicketID, &a.Kind, &a.MimeType, &a.FileName, &a.FilePath, &transcript, &a.Source, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, ErrNotFound
	}
	if err != nil {
		return a, err
	}
	if transcript.Valid {
		a.Transcript = &transcript.String
	}
	return a, nil
}

func (s *Store) SaveLead(name, email, company string) error {
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	_, err := s.db.Exec(
		"INSERT INTO leads(id,name,email,company,created_at) VALUES (?,?,?,?,?)",
		id, name, email, company, nowISO(),
	)
	return err
}

// CountLeads devuelve el total de leads capturados (G11: parte real del embudo comercial).
func (s *Store) CountLeads() (int, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM leads").Scan(&n)
	return n, err
}

// ListBuildings devuelve el catálogo de edificios — hoy son 3 filas demo sembradas en migrate()
// (ver DOCUMENTACION.md §13), no datos de un cliente real.
func (s *Store) ListBuildings() ([]Building, error) {
	rows, err := s.db.Query("SELECT id, name, tier FROM buildings ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Building{}
	for rows.Next() {
		var b Building
		if err := rows.Scan(&b.ID, &b.Name, &b.Tier); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// RecordLandingVisit suma 1 al contador de visitas del día (G11). Trackeo empezó 2026-09-23:
// no hay datos históricos de visitas previas a esa fecha, no se inventan.
func (s *Store) RecordLandingVisit() error {
	today := time.Now().UTC().Format("2006-01-02")
	_, err := s.db.Exec(`
INSERT INTO landing_visits(day, count) VALUES (?, 1)
ON CONFLICT(day) DO UPDATE SET count = count + 1`, today)
	return err
}

// LandingVisitsTotal suma todas las visitas trackeadas desde que existe la tabla.
func (s *Store) LandingVisitsTotal() (int, error) {
	var n sql.NullInt64
	if err := s.db.QueryRow("SELECT SUM(count) FROM landing_visits").Scan(&n); err != nil {
		return 0, err
	}
	return int(n.Int64), nil
}

// SetSetting guarda un par clave/valor genérico (ej. credenciales configurables desde la UI).
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		"INSERT INTO settings(key,value,updated_at) VALUES (?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at",
		key, value, nowISO(),
	)
	return err
}

// GetSetting devuelve "" si la clave no existe (no es un error: la mayoría de settings son opcionales).
func (s *Store) GetSetting(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

// DeleteSetting quita una clave (ej. "olvidar" una credencial guardada desde la UI).
func (s *Store) DeleteSetting(key string) error {
	_, err := s.db.Exec("DELETE FROM settings WHERE key = ?", key)
	return err
}

// SaveKnowledgeChunk agrega un fragmento de texto a la base de conocimiento del chatbot
// (RAG por recuperación de texto simple, ver SearchKnowledge). source identifica el
// origen (ej. "seed", "chat") para poder distinguir contenido curado de historial real.
func (s *Store) SaveKnowledgeChunk(source, text string) error {
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	_, err := s.db.Exec(
		"INSERT INTO knowledge_chunks(id,source,text,created_at) VALUES (?,?,?,?)",
		id, source, text, nowISO(),
	)
	return err
}

// CountKnowledgeChunks devuelve cuántos fragmentos hay guardados (ej. para decidir si
// hace falta insertar la semilla inicial al arrancar).
func (s *Store) CountKnowledgeChunks() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT count(*) FROM knowledge_chunks").Scan(&count)
	return count, err
}

// SearchKnowledge recupera hasta `limit` fragmentos relevantes para query mediante
// coincidencia simple por palabras clave (LIKE), sin FTS5 ni vector store: es RAG básico
// por recuperación de texto, no un modelo entrenado.
func (s *Store) SearchKnowledge(query string, limit int) ([]string, error) {
	words := strings.Fields(query)
	var keywords []string
	for _, w := range words {
		if len(w) >= 4 {
			keywords = append(keywords, w)
		}
	}
	if len(keywords) == 0 {
		return nil, nil
	}
	likeExpr := make([]string, len(keywords))
	args := make([]any, 0, len(keywords)*2+1)
	for i, kw := range keywords {
		likeExpr[i] = "(text LIKE ?)"
		args = append(args, "%"+kw+"%")
	}
	// ORDER BY cantidad de palabras que matchean (suma de los LIKE, cada uno 0/1)
	scoreExpr := strings.Join(likeExpr, " + ")
	for _, kw := range keywords {
		args = append(args, "%"+kw+"%")
	}
	sqlQuery := fmt.Sprintf(
		"SELECT text FROM knowledge_chunks WHERE %s ORDER BY (%s) DESC LIMIT ?",
		strings.Join(likeExpr, " OR "), scoreExpr,
	)
	args = append(args, limit)
	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, err
		}
		out = append(out, text)
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
