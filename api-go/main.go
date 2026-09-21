package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"todomark/api/internal/evolution"
	"todomark/api/internal/store"
)

var phoneRe = regexp.MustCompile(`^[0-9]+$`)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func main() {
	dbPath := envOr("DB_PATH", "/data/todomark.db")
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("sqlite open: %v", err)
	}
	defer st.Close()
	evo := evolution.NewFromEnv()

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
			log.Printf("aviso: webhook del bot no registrado: %v", werr)
		} else {
			log.Printf("webhook del bot registrado: %s/webhook/evolution", botURL)
		}
	}

	mux := http.NewServeMux()

	// --- Health ---
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
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

	mux.HandleFunc("POST /api/tickets", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Title       string  `json:"title"`
			Description string  `json:"description"`
			Priority    string  `json:"priority"`
			Requester   string  `json:"requester"`
			AssignedTo  *string `json:"assignedTo"`
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
		if !phoneRe.MatchString(in.Phone) {
			errJSON(w, 400, "Formato inválido: solo dígitos, código de país sin +")
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
		if err := evo.SendMessage(in.ChatID, text); err != nil {
			log.Printf("evolution send error: %v", err)
			errJSON(w, 502, "Evolution API no pudo enviar el mensaje")
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

	// --- Chatbot: menú conversacional solo para destinatarios registrados ---
	type botSession struct {
		menu   bool
		ticket string
		expiry time.Time
	}
	botSessions := sync.Map{} // chatID -> botSession

	menuText := func() string {
		return "🤖 *TodoMark Bot*\n\n*1.* Estado de mi ticket\n*2.* Descripción\n*3.* Asignado a\n*4.* Última novedad\n*5.* Salir del menú\n\n_Notas de voz: no puedo reproducirlas._"
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
		var ev struct {
			Event string `json:"event"`
			Data  struct {
				Key struct {
					RemoteJid string `json:"remoteJid"`
					FromMe    bool   `json:"fromMe"`
				} `json:"key"`
				Message struct {
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
		if json.Unmarshal(body, &ev) != nil {
			return
		}
		chat := ev.Data.Key.RemoteJid
		if chat == "" || ev.Data.Key.FromMe || ev.Event != "messages.upsert" {
			return
		}
		chat = strings.Split(chat, "@")[0]
		tickets, err := st.ListByChat(chat)
		if err != nil || len(tickets) == 0 {
			return // bot inactivo para este número
		}
		hasAudio := ev.Data.Message.AudioMessage != nil
		text := strings.TrimSpace(ev.Data.Message.Conversation)
		if text == "" && ev.Data.Message.ExtendedTextMessage != nil {
			text = strings.TrimSpace(ev.Data.Message.ExtendedTextMessage.Text)
		}
		var reply string
		if hasAudio || (text == "" && (ev.Data.Message.ImageMessage != nil || ev.Data.Message.VideoMessage != nil || ev.Data.Message.DocumentMessage != nil)) {
			kind := "archivo"
			if hasAudio {
				kind = "nota de voz"
			}
			reply = "🎙 Recibí tu " + kind + ", pero no puedo procesarlo. Escribe *menu* para ver las opciones."
		} else {
			opt := strings.ToLower(text)
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
				log.Printf("bot: no se pudo responder a %s: %v", chat, err)
			}
		}
	})

	addr := ":" + envOr("PORT", "3000")
	log.Printf("API (Go) listening on %s, db=%s", addr, dbPath)
	if err := http.ListenAndServe(addr, cors(mux)); err != nil {
		log.Fatal(err)
	}
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

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
