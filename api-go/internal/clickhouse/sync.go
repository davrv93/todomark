package clickhouse

import (
	"context"
	"log"
	"strconv"
	"time"

	"todomark/api/internal/store"
)

// Syncer vuelca periódicamente el store SQLite operacional hacia ClickHouse.
// Mismo patrón que glpi.Syncer: Run(ctx, every) con un primer sync inmediato
// y luego uno por tick, sin bloquear si el cliente no está configurado.
type Syncer struct {
	Client *Client
	Store  *store.Store
}

func (s Syncer) Run(ctx context.Context, every time.Duration) {
	if s.Client == nil || s.Store == nil || !s.Client.Configured() {
		return
	}
	if every <= 0 {
		every = 5 * time.Minute
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	s.syncOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncOnce(ctx)
		}
	}
}

// syncOnce refleja el estado actual completo de tickets/eventos en ClickHouse.
// MergeTree no soporta upsert real, así que en vez de deduplicar se trunca y
// reinserta cada ciclo: más simple y correcto para el volumen de TodoMark.
func (s Syncer) syncOnce(ctx context.Context) {
	tickets, err := s.Store.All()
	if err != nil {
		log.Printf(`{"level":"warn","msg":"clickhouse sync list failed","error":%q}`, err.Error())
		return
	}
	if err := s.Client.ReplaceTickets(ctx, tickets); err != nil {
		log.Printf(`{"level":"warn","msg":"clickhouse sync failed","error":%q}`, err.Error())
	}
}

// ReplaceTickets trunca fact_tickets/fact_events y reinserta el snapshot
// actual de tickets (y su historial de eventos) del store.
func (c *Client) ReplaceTickets(ctx context.Context, tickets []store.Ticket) error {
	conn, err := c.getConn(ctx)
	if err != nil {
		return err
	}

	if err := conn.Exec(ctx, "TRUNCATE TABLE fact_tickets"); err != nil {
		return err
	}
	ticketBatch, err := conn.PrepareBatch(ctx, "INSERT INTO fact_tickets "+
		"(TicketID, Title, Status, Priority, Requester, Email, AssignedTo, AssignedTeam, CreatedAt, UpdatedAt, ClosedAt, Channel, Category, BuildingID, SatisfactionScore)")
	if err != nil {
		return err
	}
	for _, t := range tickets {
		id, ok := parseTicketID(t.ID)
		if !ok {
			continue
		}
		if err := ticketBatch.Append(
			id,
			t.Title,
			t.Status,
			t.Priority,
			t.Requester,
			value(t.Email),
			value(t.AssignedTo),
			value(t.AssignedTeam),
			parseTime(t.CreatedAt),
			parseTime(t.UpdatedAt),
			parseNullableTime(t.ClosedAt),
			value(t.Channel),
			value(t.Category),
			value(t.BuildingID),
			nullableUint8(t.SatisfactionScore),
		); err != nil {
			return err
		}
	}
	if err := ticketBatch.Send(); err != nil {
		return err
	}

	if err := conn.Exec(ctx, "TRUNCATE TABLE fact_events"); err != nil {
		return err
	}
	eventBatch, err := conn.PrepareBatch(ctx, "INSERT INTO fact_events "+
		"(TicketID, Timestamp, User, Action, FromState, ToState, CreatedAt)")
	if err != nil {
		return err
	}
	for _, t := range tickets {
		id, ok := parseTicketID(t.ID)
		if !ok {
			continue
		}
		for _, ev := range t.History {
			ts := parseTime(ev.Timestamp)
			if err := eventBatch.Append(
				id,
				ts,
				ev.User,
				ev.Action,
				ev.From,
				ev.To,
				ts,
			); err != nil {
				return err
			}
		}
	}
	return eventBatch.Send()
}

func parseTicketID(id string) (uint64, bool) {
	n, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func parseNullableTime(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t := parseTime(*s)
	if t.IsZero() {
		return nil
	}
	return &t
}

func value(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nullableUint8(n *int) *uint8 {
	if n == nil {
		return nil
	}
	v := uint8(*n)
	return &v
}
