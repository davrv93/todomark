package glpi

import (
	"context"
	"errors"
	"log"
	"time"

	"todomark/api/internal/store"
)

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
	s.syncOnce()
	for {
		select {
		case <-ctx.Done():
			_ = s.Client.KillSession()
			return
		case <-ticker.C:
			s.syncOnce()
		}
	}
}

func (s Syncer) syncOnce() {
	remotes, err := s.Client.ListTickets()
	if err != nil {
		log.Printf(`{"level":"warn","msg":"glpi sync list failed","error":%q}`, err.Error())
		return
	}
	remoteByID := map[string]RemoteTicket{}
	for _, rt := range remotes {
		remoteByID[rt.ID] = rt
		if rt.ID == "" {
			continue
		}
		local, err := s.Store.FindByRemoteID(rt.ID)
		if errors.Is(err, store.ErrNotFound) {
			t := store.NewTicket(rt.Title, rt.Description, rt.Priority, rt.Requester, nil)
			glpiChannel := "glpi"
			t.Channel = &glpiChannel
			t.Status = rt.Status
			if rt.AssignedTo != "" {
				t.AssignedTeam = &rt.AssignedTo
			}
			if rt.Requester != "" {
				t.Email = &rt.Requester
			}
			if rt.ClosedAt != "" {
				t.ClosedAt = &rt.ClosedAt
			}
			t.RemoteID = &rt.ID
			now := time.Now().UTC().Format(time.RFC3339)
			t.LastSyncAt = &now
			if rt.UpdatedAt != "" {
				t.RemoteUpdatedAt = &rt.UpdatedAt
			}
			_ = s.Store.Create(t)
			continue
		}
		if err != nil {
			log.Printf(`{"level":"warn","msg":"glpi local lookup failed","error":%q}`, err.Error())
			continue
		}
		s.reconcile(local, rt)
	}
	locals, err := s.Store.All()
	if err != nil {
		return
	}
	for _, local := range locals {
		if local.RemoteID != nil && *local.RemoteID != "" {
			continue
		}
		rt, err := s.Client.CreateTicket(local)
		if err != nil {
			log.Printf(`{"level":"warn","msg":"glpi create remote failed","ticket_id":%q,"error":%q}`, local.ID, err.Error())
			continue
		}
		_, _ = s.Store.Update(local.ID, func(t *store.Ticket) error {
			now := time.Now().UTC().Format(time.RFC3339)
			t.RemoteID = &rt.ID
			t.LastSyncAt = &now
			if rt.UpdatedAt != "" {
				t.RemoteUpdatedAt = &rt.UpdatedAt
			}
			return nil
		})
	}
	_ = remoteByID
}

func (s Syncer) reconcile(local store.Ticket, rt RemoteTicket) {
	lastSync := value(local.LastSyncAt)
	localChanged := lastSync == "" || local.UpdatedAt > lastSync
	remoteChanged := lastSync == "" || rt.UpdatedAt > lastSync
	if localChanged && remoteChanged {
		_ = s.Store.RecordGLPIConflict(local.ID, rt.ID, local.UpdatedAt, rt.UpdatedAt, "local y remoto cambiaron desde last_sync_at")
		return
	}
	if localChanged {
		if err := s.Client.UpdateTicket(rt.ID, local); err != nil {
			log.Printf(`{"level":"warn","msg":"glpi update remote failed","ticket_id":%q,"error":%q}`, local.ID, err.Error())
			return
		}
	}
	if remoteChanged {
		_, _ = s.Store.Update(local.ID, func(t *store.Ticket) error {
			t.Title = rt.Title
			t.Description = rt.Description
			t.Status = rt.Status
			t.Priority = rt.Priority
			t.Requester = rt.Requester
			if rt.Requester != "" {
				t.Email = &rt.Requester
			}
			if rt.AssignedTo != "" {
				t.AssignedTeam = &rt.AssignedTo
			}
			if rt.ClosedAt != "" {
				t.ClosedAt = &rt.ClosedAt
			}
			return nil
		})
	}
	_, _ = s.Store.Update(local.ID, func(t *store.Ticket) error {
		now := time.Now().UTC().Format(time.RFC3339)
		t.LastSyncAt = &now
		if rt.UpdatedAt != "" {
			t.RemoteUpdatedAt = &rt.UpdatedAt
		}
		return nil
	})
}

func value(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
