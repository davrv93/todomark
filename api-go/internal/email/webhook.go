package email

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"todomark/api/internal/store"
)

var ticketRefRe = regexp.MustCompile(`(?i)\[Ticket#([^\]]+)\]`)

type Handler struct {
	Store *store.Store
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		http.Error(w, "store no configurado", http.StatusInternalServerError)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "multipart/form-data invalido", http.StatusBadRequest)
		return
	}
	eventID := firstForm(r, "message_id", "Message-Id", "message-id", "event_id")
	if eventID == "" {
		eventID = firstForm(r, "from") + "|" + firstForm(r, "subject")
	}
	fresh, err := h.Store.MarkProcessed(eventID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	if !fresh {
		return
	}

	from := strings.TrimSpace(firstForm(r, "from", "sender"))
	subject := strings.TrimSpace(firstForm(r, "subject"))
	text := strings.TrimSpace(firstForm(r, "text", "body", "plain"))
	if subject == "" {
		subject = "Correo sin asunto"
	}
	if match := ticketRefRe.FindStringSubmatch(subject); len(match) == 2 {
		id := match[1]
		_, _ = h.Store.Update(id, func(t *store.Ticket) error {
			prev := t.Description
			if text != "" {
				t.Description = strings.TrimSpace(t.Description + "\n\n--- Respuesta email ---\n" + text)
			}
			t.History = append(t.History, store.Event{
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				User:      fromOrDefault(from),
				Action:    "email_update",
				From:      prev,
				To:        t.Description,
			})
			if from != "" {
				t.Email = &from
			}
			return nil
		})
		return
	}

	t := store.NewTicket(subject, text, "medium", fromOrDefault(from), nil)
	emailChannel := "email"
	t.Channel = &emailChannel
	if from != "" {
		t.Email = &from
	}
	t.History = append(t.History, store.Event{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		User:      fromOrDefault(from),
		Action:    "email_create",
		From:      "",
		To:        fmt.Sprintf("%s: %s", subject, text),
	})
	_ = h.Store.Create(t)
}

func firstForm(r *http.Request, names ...string) string {
	for _, name := range names {
		if v := strings.TrimSpace(r.FormValue(name)); v != "" {
			return v
		}
	}
	return ""
}

func fromOrDefault(from string) string {
	if strings.TrimSpace(from) == "" {
		return "email"
	}
	return from
}
