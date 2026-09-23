package whatsapp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendTextRetries429(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatalf("missing auth")
		}
		if attempts == 1 {
			http.Error(w, "rate", http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"messages":[{"id":"1"}]}`))
	}))
	defer srv.Close()

	t.Setenv("USE_META_WHATSAPP", "true")
	t.Setenv("META_WHATSAPP_URL", srv.URL)
	t.Setenv("META_WHATSAPP_TOKEN", "token")
	t.Setenv("META_WHATSAPP_PHONE_NUMBER_ID", "123")
	client := NewFromEnv()
	if err := client.SendText("51999999999", "hola"); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d", attempts)
	}
}
