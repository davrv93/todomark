package glpi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientListTickets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apirest.php/initSession":
			if r.Header.Get("App-Token") != "app" {
				t.Fatalf("missing app token")
			}
			w.Write([]byte(`{"session_token":"session"}`))
		case "/apirest.php/Ticket":
			if r.Header.Get("Session-Token") != "session" {
				t.Fatalf("missing session token")
			}
			w.Write([]byte(`[{"id":7,"name":"Router","content":"Caido","status":"2","priority":"4","requester":"ops@example.com","assigned_to":"Redes","date_mod":"2026-09-22T00:00:00Z"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("GLPI_URL", srv.URL)
	t.Setenv("GLPI_APP_TOKEN", "app")
	t.Setenv("GLPI_USER", "user")
	t.Setenv("GLPI_PASSWORD", "pass")
	client := NewFromEnv()
	tickets, err := client.ListTickets()
	if err != nil {
		t.Fatal(err)
	}
	if len(tickets) != 1 || tickets[0].Status != "in_progress" || tickets[0].Priority != "high" {
		t.Fatalf("tickets=%+v", tickets)
	}
}
