package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"todomark/api/internal/clickhouse"
	"todomark/api/internal/deepseek"
	emailbridge "todomark/api/internal/email"
	"todomark/api/internal/evolution"
	"todomark/api/internal/gemini"
	"todomark/api/internal/glpi"
	"todomark/api/internal/hydra"
	"todomark/api/internal/presence"
	"todomark/api/internal/report"
	"todomark/api/internal/store"
	metawhatsapp "todomark/api/internal/whatsapp"
)

var phoneRe = regexp.MustCompile(`^[0-9]+$`)

// chatTargetRe acepta un número individual (solo dígitos) o el JID de un grupo
// de WhatsApp (termina en @g.us, formato Baileys/Evolution).
var chatTargetRe = regexp.MustCompile(`^[0-9]{8,15}$|^[0-9]{10,30}(-[0-9]{1,15})?@g\.us$`)

// emailRe es una validación simple (no RFC 5322 completo) para el lead form de la landing.
var emailRe = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
var requestCount uint64

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func main() {
	log.SetFlags(0)
	dbPath := envOr("DB_PATH", "/data/todomark.db")
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()
	evo := evolution.NewFromEnv()
	emailClient := emailbridge.NewClientFromEnv()
	meta := metawhatsapp.NewFromEnv()
	glpiClient := glpi.NewFromEnv()
	hydraClient := hydra.NewFromEnv()
	clickhouseClient := clickhouse.NewFromEnv()
	deepseekClient := deepseek.NewFromEnv()
	// La key también puede guardarse desde la UI (POST /api/settings/deepseek);
	// si hay una guardada, tiene prioridad sobre la variable de entorno de arranque.
	if saved, err := st.GetSetting("deepseek_api_key"); err == nil && saved != "" {
		deepseekClient.SetAPIKey(saved)
	}
	geminiClient := gemini.NewFromEnv()
	// Mismo patrón: la key guardada desde la UI (POST /api/settings/gemini) tiene
	// prioridad sobre la variable de entorno de arranque.
	if saved, err := st.GetSetting("gemini_api_key"); err == nil && saved != "" {
		geminiClient.SetAPIKey(saved)
	}
	// Semilla mínima de conocimiento para el RAG del chatbot: solo si la tabla está
	// vacía, para no duplicar contenido en cada reinicio.
	if count, err := st.CountKnowledgeChunks(); err == nil && count == 0 {
		for _, chunk := range []string{
			"TodoMark ERP ofrece integración total con JSR, escalabilidad para crecer con el negocio, UX estilo Apple, automatización de procesos y soporte 24/7.",
			"TodoMark ERP es un sistema de gestión de tickets y soporte con chatbot conversacional, integración con WhatsApp (vía Evolution API) y sincronización con GLPI.",
			"La landing de TodoMark permite solicitar una demo del ERP, dejando nombre, email y empresa en el formulario de contacto.",
			"El equipo de soporte de TodoMark responde consultas técnicas y comerciales; para casos urgentes se recomienda solicitar una demo o dejar los datos de contacto.",
		} {
			_ = st.SaveKnowledgeChunk("seed", chunk)
		}
	}
	presenceHub := presence.NewHub()
	inboxShowAll := !strings.EqualFold(envOr("INBOX_SHOW_ALL", "true"), "false")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go glpi.Syncer{Client: glpiClient, Store: st}.Run(ctx, 5*time.Minute)
	go clickhouse.Syncer{Client: clickhouseClient, Store: st}.Run(ctx, 5*time.Minute)

	// Chatbot: registrar webhook de mensajes entrantes en Evolution (con reintentos:
	// la instancia puede estar reconectando justo tras el arranque)
	if botURL := envOr("WEBHOOK_URL", ""); botURL != "" {
		var werr error
		for i := 0; i < 5; i++ {
			if werr = evo.SetWebhook(botURL + "/webhook/evolution"); werr == nil {
				break
			}
			time.Sleep(2 * time.Second)
		}
		if werr != nil {
			logJSON("warn", "webhook del bot no registrado", map[string]any{"error": werr.Error()})
		} else {
			logJSON("info", "webhook del bot registrado", map[string]any{"url": botURL + "/webhook/evolution"})
		}
	}

	mux := http.NewServeMux()

	// --- Health ---
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "todomark_http_requests_total %d\n", atomic.LoadUint64(&requestCount))
	})

	// --- Presencia en vivo (SSE): cuántos clientes conectados hay ahora ---
	mux.HandleFunc("GET /api/presence/stream", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			errJSON(w, 500, "streaming no soportado")
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := presenceHub.Add()
		defer presenceHub.Remove(ch)

		fmt.Fprintf(w, "data: %d\n\n", presenceHub.Count())
		flusher.Flush()

		for {
			select {
			case <-r.Context().Done():
				return
			case count, open := <-ch:
				if !open {
					return
				}
				fmt.Fprintf(w, "data: %d\n\n", count)
				flusher.Flush()
			}
		}
	})

	mux.Handle("POST /webhook/email", emailbridge.Handler{Store: st})

	// --- Hydra login/consent provider ---
	// Valida credenciales del lado servidor (mismo par admin/user que el login
	// de fallback en el frontend) y le dice a Hydra que acepte el login_challenge.
	mux.HandleFunc("POST /api/hydra/login", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Challenge string `json:"challenge"`
			Username  string `json:"username"`
			Password  string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Challenge == "" {
			errJSON(w, 400, "JSON inválido")
			return
		}
		if in.Username != "admin" || in.Password != "user" {
			errJSON(w, 401, "credenciales inválidas")
			return
		}
		redirectTo, err := hydraClient.AcceptLogin(in.Challenge, in.Username)
		if err != nil {
			errJSON(w, 502, "hydra no aceptó el login: "+err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"redirect_to": redirectTo})
	})

	// Primera parte (nuestro propio SPA): otorga el scope solicitado sin pantalla
	// de consentimiento manual.
	mux.HandleFunc("POST /api/hydra/consent", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Challenge string `json:"challenge"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Challenge == "" {
			errJSON(w, 400, "JSON inválido")
			return
		}
		redirectTo, err := hydraClient.AcceptConsent(in.Challenge)
		if err != nil {
			errJSON(w, 502, "hydra no aceptó el consent: "+err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"redirect_to": redirectTo})
	})

	// --- Tickets ---
	mux.HandleFunc("GET /api/tickets", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tickets, err := st.List(q.Get("status"), q.Get("priority"), q.Get("query"))
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, tickets)
	})

	mux.HandleFunc("GET /api/tickets/email/{email}", func(w http.ResponseWriter, r *http.Request) {
		tickets, err := st.ListByEmail(r.PathValue("email"))
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, tickets)
	})

	mux.HandleFunc("GET /api/tickets/team/{team}", func(w http.ResponseWriter, r *http.Request) {
		tickets, err := st.ListByTeam(r.PathValue("team"))
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, tickets)
	})

	mux.HandleFunc("GET /api/reports/summary", func(w http.ResponseWriter, r *http.Request) {
		summary, err := st.ReportSummary()
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, summary)
	})

	mux.HandleFunc("POST /api/tickets/{id}/escalate", func(w http.ResponseWriter, r *http.Request) {
		t, err := st.Update(r.PathValue("id"), func(t *store.Ticket) error {
			t.History = append(t.History, store.Event{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				User:      "cliente",
				Action:    "escalated",
				From:      t.Status,
				To:        "escalated_to_manager",
			})
			return nil
		})
		if errors.Is(err, store.ErrNotFound) {
			errJSON(w, 404, "Ticket no encontrado")
			return
		}
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, t)
	})

	mux.HandleFunc("POST /api/tickets/{id}/satisfaction", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Score int `json:"score"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			errJSON(w, 400, "JSON inválido")
			return
		}
		if in.Score < 1 || in.Score > 5 {
			errJSON(w, 400, "score debe ser 1-5")
			return
		}
		t, err := st.SetSatisfaction(r.PathValue("id"), in.Score)
		if errors.Is(err, store.ErrNotFound) {
			errJSON(w, 404, "Ticket no encontrado")
			return
		}
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, t)
	})

	mux.HandleFunc("GET /api/reports/executive", func(w http.ResponseWriter, r *http.Request) {
		summary, err := st.ExecutiveSummary()
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, summary)
	})

	mux.HandleFunc("POST /api/tickets", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Title        string  `json:"title"`
			Description  string  `json:"description"`
			Priority     string  `json:"priority"`
			Requester    string  `json:"requester"`
			AssignedTo   *string `json:"assignedTo"`
			Email        *string `json:"email"`
			AssignedTeam *string `json:"assignedTeam"`
			Category     *string `json:"category"`
			BuildingID   *string `json:"buildingId"`
			UnitID       *string `json:"unitId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			errJSON(w, 400, "JSON inválido")
			return
		}
		if in.Title == "" {
			errJSON(w, 400, "title requerido")
			return
		}
		if in.Priority == "" {
			in.Priority = "low"
		}
		if !store.ValidPriority(in.Priority) {
			errJSON(w, 400, "priority inválida")
			return
		}
		t := store.NewTicket(in.Title, in.Description, in.Priority, in.Requester, in.AssignedTo)
		t.Email = in.Email
		t.AssignedTeam = in.AssignedTeam
		t.Category = in.Category
		t.BuildingID = in.BuildingID
		t.UnitID = in.UnitID
		webChannel := "web"
		t.Channel = &webChannel
		if err := st.Create(t); err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, t)
	})

	mux.HandleFunc("GET /api/tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		t, err := st.Get(r.PathValue("id"))
		if errors.Is(err, store.ErrNotFound) {
			errJSON(w, 404, "Ticket no encontrado")
			return
		}
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, t)
	})

	// Query param en vez de /api/tickets/{id}/attachments: ese patrón choca con
	// /api/tickets/email/{email} en net/http.ServeMux (Go 1.22+ detecta la ambigüedad
	// {id} vs "email" en la misma posición y hace panic al arrancar).
	mux.HandleFunc("GET /api/attachments", func(w http.ResponseWriter, r *http.Request) {
		ticketID := r.URL.Query().Get("ticketId")
		if ticketID == "" {
			errJSON(w, 400, "ticketId requerido")
			return
		}
		list, err := st.ListAttachments(ticketID)
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, list)
	})

	// Sirve el archivo crudo de un adjunto (foto/audio) — no requiere auth extra, mismo
	// nivel de exposición que el resto de este gateway (REQUIRE_API_AUTH ya lo cubre si está on).
	mux.HandleFunc("GET /api/attachments/{id}/file", func(w http.ResponseWriter, r *http.Request) {
		a, err := st.GetAttachment(r.PathValue("id"))
		if errors.Is(err, store.ErrNotFound) {
			errJSON(w, 404, "Adjunto no encontrado")
			return
		}
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		if a.MimeType != "" {
			w.Header().Set("Content-Type", a.MimeType)
		}
		http.ServeFile(w, r, a.FilePath)
	})

	mux.HandleFunc("PUT /api/tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			errJSON(w, 400, "JSON inválido")
			return
		}
		// Campos editables: los de identidad, fechas, history y status se ignoran
		// (status cambia solo vía /transition para respetar la matriz)
		delete(in, "id")
		delete(in, "createdAt")
		delete(in, "updatedAt")
		delete(in, "history")
		delete(in, "status")
		delete(in, "whatsappChatId")
		if p, ok := in["priority"].(string); ok && !store.ValidPriority(p) {
			errJSON(w, 400, "priority inválida")
			return
		}
		if s, ok := in["status"].(string); ok && !store.ValidStatus(s) {
			errJSON(w, 400, "status inválido")
			return
		}
		t, err := st.Update(r.PathValue("id"), func(t *store.Ticket) error {
			return applyPatch(t, in)
		})
		if errors.Is(err, store.ErrNotFound) {
			errJSON(w, 404, "Ticket no encontrado")
			return
		}
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, t)
	})

	mux.HandleFunc("DELETE /api/tickets/{id}", func(w http.ResponseWriter, r *http.Request) {
		if err := st.Delete(r.PathValue("id")); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				errJSON(w, 404, "Ticket no encontrado")
				return
			}
			errJSON(w, 500, err.Error())
			return
		}
		w.WriteHeader(204)
	})

	// --- Transición de estado ---
	mux.HandleFunc("POST /api/tickets/{id}/transition", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			NewStatus string `json:"newStatus"`
			User      string `json:"user"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			errJSON(w, 400, "JSON inválido")
			return
		}
		if !store.ValidStatus(in.NewStatus) {
			errJSON(w, 400, "status inválido")
			return
		}
		if in.User == "" {
			in.User = "system"
		}
		id := r.PathValue("id")
		// La validación ocurre dentro del Update atómico para evitar carreras
		t, err := st.Update(id, func(t *store.Ticket) error {
			if !store.AllowedTransition(t.Status, in.NewStatus) {
				return fmt.Errorf("%w: de %s a %s", store.ErrInvalidTransition, t.Status, in.NewStatus)
			}
			from := t.Status
			t.Status = in.NewStatus
			if in.NewStatus == "closed" {
				now := time.Now().UTC().Format(time.RFC3339)
				t.ClosedAt = &now
			} else if from == "closed" {
				t.ClosedAt = nil
			}
			t.History = append(t.History, store.Event{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				User:      in.User,
				Action:    "status_change",
				From:      from,
				To:        in.NewStatus,
			})
			return nil
		})
		if errors.Is(err, store.ErrNotFound) {
			errJSON(w, 404, "Ticket no encontrado")
			return
		}
		if errors.Is(err, store.ErrInvalidTransition) {
			errJSON(w, 409, err.Error())
			return
		}
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, t)
	})

	// --- Destinatario WhatsApp ---
	mux.HandleFunc("POST /api/tickets/{id}/recipient", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Phone string `json:"phone"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			errJSON(w, 400, "JSON inválido")
			return
		}
		if !chatTargetRe.MatchString(in.Phone) {
			errJSON(w, 400, "Formato inválido: número (solo dígitos, código de país sin +) o JID de grupo terminado en @g.us")
			return
		}
		id := r.PathValue("id")
		cur, err := st.Get(id)
		if errors.Is(err, store.ErrNotFound) {
			errJSON(w, 404, "Ticket no encontrado")
			return
		}
		prev := ""
		if cur.WhatsappChatID != nil {
			prev = *cur.WhatsappChatID
		}
		t, err := st.Update(id, func(t *store.Ticket) error {
			phone := in.Phone
			t.WhatsappChatID = &phone
			t.History = append(t.History, store.Event{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				User:      "system",
				Action:    "set_recipient",
				From:      prev,
				To:        in.Phone,
			})
			return nil
		})
		if errors.Is(err, store.ErrNotFound) {
			errJSON(w, 404, "Ticket no encontrado")
			return
		}
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, t)
	})

	// --- Notificación (POST /api/notify) ---
	mux.HandleFunc("POST /api/notify", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			TicketID string `json:"ticketId"`
			Event    string `json:"event"`
			ChatID   string `json:"chatId"`
			Message  string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			errJSON(w, 400, "JSON inválido")
			return
		}
		if in.ChatID == "" {
			errJSON(w, 400, "chatId requerido: asigna un destinatario al ticket")
			return
		}
		text := in.Message
		if text == "" {
			text = fmt.Sprintf("Ticket %s - %s", in.TicketID, in.Event)
		}
		if meta.Enabled() {
			if err := meta.SendText(in.ChatID, text); err != nil {
				logJSON("warn", "meta whatsapp send failed", map[string]any{"error": err.Error()})
				if terr := meta.SendTemplate(in.ChatID, []string{in.TicketID, in.Event}); terr != nil {
					errJSON(w, 502, "Meta WhatsApp no pudo enviar el mensaje")
					return
				}
			}
		} else {
			if err := evo.SendMessage(in.ChatID, text); err != nil {
				logJSON("warn", "evolution send error", map[string]any{"error": err.Error()})
				errJSON(w, 502, "Evolution API no pudo enviar el mensaje")
				return
			}
		}
		writeJSON(w, 200, map[string]string{"status": "sent"})
	})

	mux.HandleFunc("POST /api/notify/email", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			To      string `json:"to"`
			Subject string `json:"subject"`
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			errJSON(w, 400, "JSON inválido")
			return
		}
		if in.To == "" {
			errJSON(w, 400, "to requerido")
			return
		}
		if err := emailClient.Send(in.To, in.Subject, in.Message); err != nil {
			errJSON(w, 502, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"status": "sent"})
	})

	// --- Estado de la instancia WhatsApp ---
	mux.HandleFunc("GET /api/whatsapp/status", func(w http.ResponseWriter, r *http.Request) {
		state, err := evo.ConnectionStatus()
		if err != nil {
			var he *evolution.ErrorHTTP
			if errors.As(err, &he) && he.Status == 404 {
				errJSON(w, 404, "Instancia no encontrada")
				return
			}
			errJSON(w, 502, "No se pudo consultar el estado de la instancia")
			return
		}
		writeJSON(w, 200, map[string]string{"state": state})
	})

	// --- QR para emparejar (POST /api/whatsapp/qr) ---
	mux.HandleFunc("POST /api/whatsapp/qr", func(w http.ResponseWriter, r *http.Request) {
		qr, err := evo.Connect()
		if err != nil {
			var he *evolution.ErrorHTTP
			if errors.As(err, &he) && he.Status == 404 {
				if err := evo.CreateInstance(); err != nil {
					errJSON(w, 502, "No se pudo crear la instancia de WhatsApp")
					return
				}
				// La instancia tarda un instante en registrarse: reintentar
				for i := 0; i < 3; i++ {
					time.Sleep(500 * time.Millisecond)
					qr, err = evo.Connect()
					if err == nil && qr != nil {
						break
					}
				}
				if err != nil {
					errJSON(w, 502, "No se pudo generar el QR")
					return
				}
			} else {
				errJSON(w, 502, "No se pudo generar el QR")
				return
			}
		}
		if qr == nil {
			state, serr := evo.ConnectionStatus()
			if serr == nil && state == "open" {
				writeJSON(w, 200, map[string]bool{"connected": true})
				return
			}
			errJSON(w, 502, "La instancia no devolvió QR; reintenta en unos segundos")
			return
		}
		writeJSON(w, 200, map[string]any{"connected": false, "qr": *qr})
	})

	// --- Pairing code (alternativa al QR) ---
	mux.HandleFunc("POST /api/whatsapp/pair", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Phone string `json:"phone"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			errJSON(w, 400, "JSON inválido")
			return
		}
		if !phoneRe.MatchString(in.Phone) {
			errJSON(w, 400, "Formato inválido: solo dígitos, código de país sin +")
			return
		}
		code, err := evo.PairingCode(in.Phone)
		if err != nil {
			errJSON(w, 502, err.Error())
			return
		}
		writeJSON(w, 200, map[string]string{"pairingCode": code})
	})

	// --- Desvincular instancia (logout, sin borrar la instancia) ---
	mux.HandleFunc("POST /api/whatsapp/logout", func(w http.ResponseWriter, r *http.Request) {
		if err := evo.Logout(); err != nil {
			var he *evolution.ErrorHTTP
			if errors.As(err, &he) && he.Status == 404 {
				errJSON(w, 404, "Instancia no encontrada")
				return
			}
			errJSON(w, 502, "No se pudo desvincular la instancia")
			return
		}
		writeJSON(w, 200, map[string]string{"state": "close"})
	})

	// --- Grupos de WhatsApp disponibles (para elegir destino en vez de tipear el JID) ---
	mux.HandleFunc("GET /api/whatsapp/groups", func(w http.ResponseWriter, r *http.Request) {
		groups, err := evo.FetchGroups()
		if err != nil {
			errJSON(w, 502, "No se pudieron obtener los grupos (¿la instancia está vinculada?)")
			return
		}
		writeJSON(w, 200, groups)
	})

	// --- Estado de la integración GLPI (visibilidad en la UI) ---
	mux.HandleFunc("GET /api/glpi/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]bool{"configured": glpiClient.Configured()})
	})

	// --- Chatbot: menú conversacional + registro de ticket nuevo (2026-09-23) ---
	type botSession struct {
		menu     bool
		ticket   string
		expiry   time.Time
		creating string // "" | "awaiting_description" | "awaiting_attachment"
		draft    string // id del ticket recién creado, mientras se espera el adjunto
	}
	botSessions := sync.Map{} // chatID -> botSession
	attachDir := filepath.Join(filepath.Dir(dbPath), "attachments")
	if err := os.MkdirAll(attachDir, 0755); err != nil {
		logJSON("warn", "no se pudo crear el directorio de adjuntos", map[string]any{"error": err.Error()})
	}

	isIncidentKeyword := func(opt string) bool {
		for _, kw := range []string{"incidente", "problema", "reportar", "nuevo ticket", "tengo un"} {
			if strings.Contains(opt, kw) {
				return true
			}
		}
		return false
	}

	menuText := func() string {
		return "🤖 *TodoMark Bot*\n\n*1.* Estado de mi ticket\n*2.* Descripción\n*3.* Asignado a\n*4.* Última novedad\n*5.* Salir del menú\n\n" +
			"Escribe *reporte* en cualquier momento para recibir el resumen ejecutivo (texto + PDF).\n" +
			"Escribe *incidente* para reportar un problema nuevo.\n\n_Notas de voz: solo se transcriben al reportar un incidente nuevo._"
	}
	answer := func(t store.Ticket, opt string) string {
		switch opt {
		case "1":
			return "📋 *" + t.Title + "*\nEstado: *" + t.Status + "* (prioridad " + t.Priority + ")"
		case "2":
			return "📄 *" + t.Title + "*\n\n" + t.Description
		case "3":
			if t.AssignedTo != nil && *t.AssignedTo != "" {
				return "👤 Asignado a: *" + *t.AssignedTo + "*"
			}
			return "👤 Sin asignar todavía."
		case "4":
			if len(t.History) == 0 {
				return "🗓 Sin novedades registradas."
			}
			ev := t.History[len(t.History)-1]
			return "🗓 Última novedad:\n" + ev.Timestamp + " - " + ev.Action + " (" + ev.From + " → " + ev.To + ")"
		}
		return ""
	}

	mux.HandleFunc("POST /webhook/evolution", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200) // ack inmediato a Evolution
		var envelope struct {
			Event string          `json:"event"`
			Data  json.RawMessage `json:"data"`
		}
		var ev struct {
			Event string `json:"event"`
			Data  struct {
				Key struct {
					RemoteJid string `json:"remoteJid"`
					FromMe    bool   `json:"fromMe"`
				} `json:"key"`
				PushName string `json:"pushName"`
				Message  struct {
					Conversation        string `json:"conversation"`
					ExtendedTextMessage *struct {
						Text string `json:"text"`
					} `json:"extendedTextMessage"`
					AudioMessage    *map[string]any `json:"audioMessage"`
					ImageMessage    *map[string]any `json:"imageMessage"`
					VideoMessage    *map[string]any `json:"videoMessage"`
					DocumentMessage *map[string]any `json:"documentMessage"`
				} `json:"message"`
			} `json:"data"`
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			return
		}
		if json.Unmarshal(body, &envelope) != nil || json.Unmarshal(body, &ev) != nil {
			return
		}
		rawData := envelope.Data // objeto completo {key, message, ...} — lo que pide Evolution para descargar medios
		fullChat := ev.Data.Key.RemoteJid
		if fullChat == "" || ev.Data.Key.FromMe || ev.Event != "messages.upsert" {
			return
		}
		chat := strings.Split(fullChat, "@")[0]
		hasAudio := ev.Data.Message.AudioMessage != nil
		hasImage := ev.Data.Message.ImageMessage != nil
		text := strings.TrimSpace(ev.Data.Message.Conversation)
		if text == "" && ev.Data.Message.ExtendedTextMessage != nil {
			text = strings.TrimSpace(ev.Data.Message.ExtendedTextMessage.Text)
		}

		// Bandeja (Fase 4A): persistir el mensaje entrante, ligado o no a un ticket.
		if inboxShowAll || chatLinkedToTicket(st, chat, fullChat) {
			persistText := text
			if persistText == "" {
				persistText = "[adjunto]"
				if hasAudio {
					persistText = "[nota de voz]"
				}
			}
			if err := st.SaveMessage(&store.Message{ChatID: fullChat, ChatName: ev.Data.PushName, Direction: "in", Text: persistText}); err != nil {
				logJSON("warn", "no se pudo guardar mensaje en la bandeja", map[string]any{"chat": fullChat, "error": err.Error()})
			}
		}

		tickets, err := st.ListByChat(chat)
		if err != nil {
			return
		}

		// saveIncomingAttachment descarga (Evolution getBase64FromMediaMessage), guarda en disco
		// bajo attachDir, transcribe si es audio y Gemini está configurado, e inserta en `attachments`.
		saveIncomingAttachment := func(ticketID, kind string) (store.Attachment, error) {
			media, err := evo.GetMediaBase64(rawData)
			if err != nil {
				return store.Attachment{}, err
			}
			raw, err := base64.StdEncoding.DecodeString(media.Base64)
			if err != nil {
				return store.Attachment{}, err
			}
			id := fmt.Sprintf("%d", time.Now().UnixNano())
			fileName := media.FileName
			if fileName == "" {
				fileName = id
			}
			ticketDir := filepath.Join(attachDir, ticketID)
			if err := os.MkdirAll(ticketDir, 0755); err != nil {
				return store.Attachment{}, err
			}
			filePath := filepath.Join(ticketDir, id+"_"+fileName)
			if err := os.WriteFile(filePath, raw, 0644); err != nil {
				return store.Attachment{}, err
			}
			a := store.Attachment{
				ID: id, TicketID: ticketID, Kind: kind, MimeType: media.MimeType, FileName: fileName,
				FilePath: filePath, Source: "whatsapp", CreatedAt: time.Now().UTC().Format(time.RFC3339),
			}
			if kind == "audio" && geminiClient.Configured() {
				if transcript, terr := geminiClient.Transcribe(raw, media.MimeType); terr == nil && transcript != "" {
					a.Transcript = &transcript
				} else if terr != nil {
					logJSON("warn", "no se pudo transcribir nota de voz", map[string]any{"error": terr.Error()})
				}
			}
			if err := st.SaveAttachment(&a); err != nil {
				return a, err
			}
			return a, nil
		}

		var reply string
		if val, ok := botSessions.Load(chat); ok {
			s := val.(botSession)
			expired := time.Now().After(s.expiry)
			if !expired && s.creating == "awaiting_description" && text != "" {
				title := text
				if len(title) > 60 {
					title = title[:60] + "…"
				}
				channel := "whatsapp"
				t := store.NewTicket(title, text, "medium", ev.Data.PushName, nil)
				t.Channel = &channel
				t.WhatsappChatID = &chat
				if err := st.Create(t); err != nil {
					logJSON("warn", "no se pudo crear ticket desde whatsapp", map[string]any{"chat": chat, "error": err.Error()})
					reply = "⚠️ No pude registrar tu incidente, intenta de nuevo en un momento."
					botSessions.Delete(chat)
				} else {
					reply = "✅ Registré tu incidente como *ticket " + t.ID + "*.\n\n" +
						"¿Querés adjuntar una foto o nota de voz? Mandala ahora, o escribí *listo* si no hace falta."
					botSessions.Store(chat, botSession{creating: "awaiting_attachment", draft: t.ID, expiry: time.Now().Add(10 * time.Minute)})
				}
			} else if !expired && s.creating == "awaiting_attachment" {
				lower := strings.ToLower(text)
				if hasImage || hasAudio {
					kind := "photo"
					label := "📷 Foto"
					if hasAudio {
						kind = "audio"
						label = "🎙 Nota de voz"
					}
					att, aerr := saveIncomingAttachment(s.draft, kind)
					if aerr != nil {
						logJSON("warn", "no se pudo guardar adjunto de whatsapp", map[string]any{"chat": chat, "ticket": s.draft, "error": aerr.Error()})
						reply = "⚠️ No pude guardar ese adjunto. Podés reintentar o escribir *listo* para terminar."
					} else if att.Transcript != nil {
						reply = label + " guardada en el ticket " + s.draft + ".\n📝 Transcripción: _" + *att.Transcript + "_\n\n¿Algo más? Mandalo, o escribí *listo*."
					} else {
						reply = label + " guardada en el ticket " + s.draft + ". ¿Algo más? Mandalo, o escribí *listo*."
					}
				} else if lower == "listo" || lower == "no" || lower == "gracias" || lower == "fin" || lower == "nada" {
					reply = "👍 Listo, tu ticket *" + s.draft + "* quedó registrado. Escribe *menu* para verlo."
					botSessions.Delete(chat)
				} else {
					reply = "Mandá una foto o nota de voz, o escribí *listo* para terminar sin adjuntar."
				}
			}
		}

		if reply != "" {
			// ya se armó la respuesta arriba (flujo de creación de ticket)
		} else if hasAudio || (text == "" && (hasImage || ev.Data.Message.VideoMessage != nil || ev.Data.Message.DocumentMessage != nil)) {
			kind := "archivo"
			if hasAudio {
				kind = "nota de voz"
			}
			reply = "🎙 Recibí tu " + kind + ", pero no puedo procesarlo acá. Escribe *incidente* para reportar un problema nuevo (ahí sí puedo guardar fotos/audio)."
		} else {
			opt := strings.ToLower(text)
			if (len(tickets) == 0 && text != "") || isIncidentKeyword(opt) {
				// Contacto nuevo (sin tickets) o alguien con tickets que quiere reportar otro
				// incidente: arranca el flujo de creación en 2 pasos (describir → adjuntar).
				reply = "🤖 *TodoMark Bot*\n\nContame qué pasó — describí el incidente en un mensaje."
				botSessions.Store(chat, botSession{creating: "awaiting_description", expiry: time.Now().Add(10 * time.Minute)})
			} else if opt == "reporte" || opt == "reportes" {
				// Comando global: funciona en cualquier momento, no requiere estar en el menú.
				// Mismos datos que GET /api/reports/executive — nada se calcula distinto acá.
				summary, err := st.ExecutiveSummary()
				if err != nil {
					logJSON("warn", "reporte: no se pudo calcular resumen", map[string]any{"error": err.Error()})
					return
				}
				if err := evo.SendMessage(chat, report.Text(summary)); err != nil {
					logJSON("warn", "reporte: no se pudo enviar texto", map[string]any{"chat": chat, "error": err.Error()})
					return
				}
				pdfBytes, err := report.PDF(summary)
				if err != nil {
					logJSON("warn", "reporte: no se pudo generar PDF", map[string]any{"error": err.Error()})
					return
				}
				if err := evo.SendDocument(chat, "reporte-todomark.pdf", "application/pdf", "Reporte ejecutivo TodoMark", pdfBytes); err != nil {
					logJSON("warn", "reporte: no se pudo enviar PDF", map[string]any{"chat": chat, "error": err.Error()})
				}
				return
			}
			if val, ok := botSessions.Load(chat); ok {
				s := val.(botSession)
				if time.Now().After(s.expiry) {
					botSessions.Delete(chat)
				} else if opt == "5" {
					botSessions.Delete(chat)
					reply = "👋 Menú cerrado. Escribe *menu* para abrirlo de nuevo."
				} else {
					if s.ticket != "" {
						for _, t := range tickets {
							if t.ID == s.ticket {
								if opt == "menu" {
									reply = menuText()
								} else if a := answer(t, opt); a != "" {
									reply = a
								}
								break
							}
						}
					} else {
						n, _ := strconv.Atoi(opt)
						if n >= 1 && n <= len(tickets) {
							s.ticket = tickets[n-1].ID
							botSessions.Store(chat, s)
							reply = "✅ *" + tickets[n-1].Title + "*\n\n" + menuText()
						} else if opt == "menu" {
							reply = menuText()
						}
					}
				}
			}
			if reply == "" {
				if len(tickets) == 1 {
					botSessions.Store(chat, botSession{menu: true, ticket: tickets[0].ID, expiry: time.Now().Add(10 * time.Minute)})
					reply = "🤖 *TodoMark Bot*\n\nTicket: *" + tickets[0].Title + "*\n\n" + menuText()
				} else if opt == "menu" || opt == "hola" {
					var sb strings.Builder
					sb.WriteString("🤖 *TodoMark Bot*\n\n¿Qué ticket consultas?\n")
					for i, t := range tickets {
						fmt.Fprintf(&sb, "\n*%d.* %s (%s)", i+1, t.Title, t.Status)
					}
					sb.WriteString("\n\nResponde con el número. _Menú activo 10 min._")
					reply = sb.String()
					botSessions.Store(chat, botSession{menu: true, expiry: time.Now().Add(10 * time.Minute)})
				}
			}
		}
		if reply != "" {
			if err := evo.SendMessage(chat, reply); err != nil {
				logJSON("warn", "bot no pudo responder", map[string]any{"chat": chat, "error": err.Error()})
			} else if err := st.SaveMessage(&store.Message{ChatID: fullChat, ChatName: ev.Data.PushName, Direction: "out", Text: reply}); err != nil {
				logJSON("warn", "no se pudo guardar respuesta del bot en la bandeja", map[string]any{"chat": fullChat, "error": err.Error()})
			}
		}
	})

	// --- Bandeja de mensajes (Fase 4A) ---
	mux.HandleFunc("GET /api/messages/threads", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		threads, err := st.ListThreads(limit, r.URL.Query().Get("before"))
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, threads)
	})

	mux.HandleFunc("GET /api/messages/threads/{chatId}", func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		msgs, err := st.ListMessages(r.PathValue("chatId"), limit, r.URL.Query().Get("before"))
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, msgs)
	})

	mux.HandleFunc("POST /api/messages", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			To   string `json:"to"`
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.To == "" || in.Text == "" {
			errJSON(w, 400, "JSON inválido: requiere 'to' y 'text'")
			return
		}
		if !chatTargetRe.MatchString(in.To) {
			errJSON(w, 400, "Formato inválido: número (solo dígitos, código de país sin +) o JID de grupo terminado en @g.us")
			return
		}
		if err := evo.SendMessage(in.To, in.Text); err != nil {
			errJSON(w, 502, "Evolution API no pudo enviar el mensaje")
			return
		}
		// Los mensajes entrantes llegan con el JID completo (…@s.whatsapp.net o …@g.us);
		// se normaliza acá para que un número individual quede en el mismo hilo.
		chatID := in.To
		if !strings.Contains(chatID, "@") {
			chatID += "@s.whatsapp.net"
		}
		msg := &store.Message{ChatID: chatID, Direction: "out", Text: in.Text}
		if err := st.SaveMessage(msg); err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, msg)
	})

	// --- Landing: captura de leads (persistencia local, sin CRM externo) ---
	mux.HandleFunc("POST /api/leads", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Name    string `json:"name"`
			Email   string `json:"email"`
			Company string `json:"company"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			errJSON(w, 400, "JSON inválido")
			return
		}
		in.Name = strings.TrimSpace(in.Name)
		in.Email = strings.TrimSpace(in.Email)
		in.Company = strings.TrimSpace(in.Company)
		if in.Name == "" || in.Email == "" {
			errJSON(w, 400, "name y email son requeridos")
			return
		}
		if !emailRe.MatchString(in.Email) {
			errJSON(w, 400, "email inválido")
			return
		}
		if err := st.SaveLead(in.Name, in.Email, in.Company); err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 201, map[string]string{"status": "saved"})
	})

	// POST /api/landing/visit: contador simple de visitas a la landing (G11 — embudo comercial).
	// Trackeo empezó 2026-09-23, sin datos históricos previos (no se inventan).
	mux.HandleFunc("POST /api/landing/visit", func(w http.ResponseWriter, r *http.Request) {
		if err := st.RecordLandingVisit(); err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 204, nil)
	})

	// GET /api/buildings: catálogo de edificios — hoy son 3 filas DEMO (ver DOCUMENTACION.md §13).
	mux.HandleFunc("GET /api/buildings", func(w http.ResponseWriter, r *http.Request) {
		buildings, err := st.ListBuildings()
		if err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, buildings)
	})

	// --- Landing: chatbot IA (Gemini como primario, DeepSeek como fallback), con
	// clasificador de intención y recuperación de contexto tipo RAG sobre knowledge_chunks
	// (ver internal/gemini, internal/deepseek). Inerte (fallback textual) si ninguno está configurado.
	mux.HandleFunc("POST /api/chat", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Message string `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Message) == "" {
			errJSON(w, 400, "JSON inválido: requiere 'message'")
			return
		}
		message := strings.TrimSpace(in.Message)

		category := ""
		if geminiClient.Configured() {
			if cat, err := geminiClient.Classify(message); err == nil {
				category = cat
			}
		}

		if !geminiClient.Configured() && !deepseekClient.Configured() {
			writeJSON(w, 200, map[string]string{"reply": "El asistente todavía no está configurado. Deja tu contacto en el formulario y te escribimos."})
			return
		}

		var reply string
		var err error
		if geminiClient.Configured() {
			prompt := gemini.DefaultSystemPrompt()
			if chunks, serr := st.SearchKnowledge(message, 3); serr == nil && len(chunks) > 0 {
				prompt += "\n\nContexto relevante:\n- " + strings.Join(chunks, "\n- ")
			}
			reply, err = geminiClient.Chat(prompt, message)
		} else {
			reply, err = deepseekClient.Chat(message)
		}
		if err != nil {
			errJSON(w, 502, "No se pudo contactar al asistente de IA")
			return
		}

		// La base de conocimiento crece con el uso real: más historial disponible para
		// recuperar en futuras conversaciones (no es reentrenamiento de ningún modelo).
		_ = st.SaveKnowledgeChunk("chat", "Usuario: "+message+"\nAsistente: "+reply)

		out := map[string]string{"reply": reply}
		if category != "" {
			out["category"] = category
		}
		writeJSON(w, 200, out)
	})

	// --- Configuración de DeepSeek desde la UI (en vez de solo variable de entorno) ---
	mux.HandleFunc("GET /api/settings/deepseek", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]bool{"configured": deepseekClient.Configured()})
	})
	mux.HandleFunc("POST /api/settings/deepseek", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			APIKey string `json:"apiKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.APIKey) == "" {
			errJSON(w, 400, "JSON inválido: requiere 'apiKey'")
			return
		}
		if err := st.SetSetting("deepseek_api_key", strings.TrimSpace(in.APIKey)); err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		deepseekClient.SetAPIKey(strings.TrimSpace(in.APIKey))
		writeJSON(w, 200, map[string]bool{"configured": true})
	})
	mux.HandleFunc("DELETE /api/settings/deepseek", func(w http.ResponseWriter, r *http.Request) {
		if err := st.DeleteSetting("deepseek_api_key"); err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		deepseekClient.SetAPIKey("")
		w.WriteHeader(204)
	})

	// --- Configuración de Gemini desde la UI (en vez de solo variable de entorno) ---
	mux.HandleFunc("GET /api/settings/gemini", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]bool{"configured": geminiClient.Configured()})
	})
	mux.HandleFunc("POST /api/settings/gemini", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			APIKey string `json:"apiKey"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.APIKey) == "" {
			errJSON(w, 400, "JSON inválido: requiere 'apiKey'")
			return
		}
		if err := st.SetSetting("gemini_api_key", strings.TrimSpace(in.APIKey)); err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		geminiClient.SetAPIKey(strings.TrimSpace(in.APIKey))
		writeJSON(w, 200, map[string]bool{"configured": true})
	})
	mux.HandleFunc("DELETE /api/settings/gemini", func(w http.ResponseWriter, r *http.Request) {
		if err := st.DeleteSetting("gemini_api_key"); err != nil {
			errJSON(w, 500, err.Error())
			return
		}
		geminiClient.SetAPIKey("")
		w.WriteHeader(204)
	})

	addr := ":" + envOr("PORT", "3000")
	logJSON("info", "API Go listening", map[string]any{"addr": addr, "db": dbPath})
	handler := withMetrics(withAuth(cors(mux)))
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}

// chatLinkedToTicket revisa si `chat` está ligado a algún ticket, probando tanto
// la forma corta (individuales, sin sufijo @s.whatsapp.net) como el JID completo
// (grupos, con @g.us) — son las dos formas en que whatsapp_chat_id se guarda hoy.
func chatLinkedToTicket(st *store.Store, chat, fullChat string) bool {
	if fullChat != chat {
		if tickets, err := st.ListByChat(fullChat); err == nil && len(tickets) > 0 {
			return true
		}
	}
	tickets, err := st.ListByChat(chat)
	return err == nil && len(tickets) > 0
}

func applyPatch(t *store.Ticket, in map[string]any) error {
	for k, v := range in {
		switch k {
		case "title":
			if s, ok := v.(string); ok {
				t.Title = s
			}
		case "description":
			if s, ok := v.(string); ok {
				t.Description = s
			}
		case "priority":
			if s, ok := v.(string); ok {
				t.Priority = s
			}
		case "requester":
			if s, ok := v.(string); ok {
				t.Requester = s
			}
		case "assignedTo":
			if s, ok := v.(string); ok {
				t.AssignedTo = &s
			} else if v == nil {
				t.AssignedTo = nil
			}
		case "email":
			if s, ok := v.(string); ok {
				t.Email = &s
			} else if v == nil {
				t.Email = nil
			}
		case "assignedTeam":
			if s, ok := v.(string); ok {
				t.AssignedTeam = &s
			} else if v == nil {
				t.AssignedTeam = nil
			}
		case "closedAt":
			if s, ok := v.(string); ok {
				t.ClosedAt = &s
			} else if v == nil {
				t.ClosedAt = nil
			}
		case "whatsappChatId":
			if s, ok := v.(string); ok {
				t.WhatsappChatID = &s
			} else if v == nil {
				t.WhatsappChatID = nil
			}
		}
	}
	return nil
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint64(&requestCount, 1)
		start := time.Now()
		next.ServeHTTP(w, r)
		logJSON("info", "request", map[string]any{
			"method":      r.Method,
			"path":        r.URL.Path,
			"duration_ms": time.Since(start).Milliseconds(),
		})
	})
}

type ctxKey string

const rolesCtxKey ctxKey = "todomark_roles"

func withAuth(next http.Handler) http.Handler {
	require := strings.EqualFold(os.Getenv("REQUIRE_API_AUTH"), "true")
	apiToken := os.Getenv("API_BEARER_TOKEN")
	introspectionURL := os.Getenv("HYDRA_INTROSPECTION_URL")
	clientID := os.Getenv("HYDRA_CLIENT_ID")
	clientSecret := os.Getenv("HYDRA_CLIENT_SECRET")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || !strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/api/hydra/") || !require {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			errJSON(w, 401, "token requerido")
			return
		}
		var roles []string
		authenticated := false
		if apiToken != "" && token == apiToken {
			authenticated = true
			roles = []string{"admin"}
		} else if introspectionURL != "" {
			if ok, tokenRoles := introspectToken(introspectionURL, clientID, clientSecret, token); ok {
				authenticated = true
				roles = tokenRoles
			}
		}
		if !authenticated {
			errJSON(w, 401, "token inválido")
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), rolesCtxKey, roles))
		if hasRole(r, "viewer") && r.Method != http.MethodGet {
			errJSON(w, 403, "rol viewer es solo lectura")
			return
		}
		if hasRole(r, "agent") && r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/tickets/") {
			errJSON(w, 403, "rol agent no puede eliminar tickets")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// hasRole indica si la request autenticada tiene alguno de los roles dados.
// Con auth deshabilitada (REQUIRE_API_AUTH != true) devuelve siempre true,
// para no alterar el comportamiento por defecto.
func hasRole(r *http.Request, allowed ...string) bool {
	if !strings.EqualFold(os.Getenv("REQUIRE_API_AUTH"), "true") {
		return true
	}
	roles, _ := r.Context().Value(rolesCtxKey).([]string)
	for _, role := range roles {
		for _, a := range allowed {
			if role == a {
				return true
			}
		}
	}
	return false
}

func introspectToken(url, clientID, clientSecret, token string) (bool, []string) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString("token="+token))
	if err != nil {
		return false, nil
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if clientID != "" || clientSecret != "" {
		req.SetBasicAuth(clientID, clientSecret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return false, nil
	}
	var out struct {
		Active bool   `json:"active"`
		Scope  string `json:"scope"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out) != nil || !out.Active {
		return false, nil
	}
	var roles []string
	if out.Scope != "" {
		roles = strings.Fields(out.Scope)
	}
	return true, roles
}

func logJSON(level, msg string, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["level"] = level
	fields["msg"] = msg
	fields["ts"] = time.Now().UTC().Format(time.RFC3339)
	raw, _ := json.Marshal(fields)
	log.Println(string(raw))
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
