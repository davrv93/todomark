package email

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"todomark/api/internal/store"
)

func TestWebhookCreatesTicketOnce(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := Handler{Store: st}
	body, contentType := multipartBody(map[string]string{
		"message_id": "msg-1",
		"from":       "ana@example.com",
		"subject":    "Impresora detenida",
		"text":       "No imprime",
	})
	req := httptest.NewRequest(http.MethodPost, "/webhook/email", body)
	req.Header.Set("Content-Type", contentType)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
	body, contentType = multipartBody(map[string]string{
		"message_id": "msg-1",
		"from":       "ana@example.com",
		"subject":    "Impresora detenida",
		"text":       "No imprime",
	})
	req = httptest.NewRequest(http.MethodPost, "/webhook/email", body)
	req.Header.Set("Content-Type", contentType)
	h.ServeHTTP(httptest.NewRecorder(), req)
	tickets, err := st.ListByEmail("ana@example.com")
	if err != nil || len(tickets) != 1 {
		t.Fatalf("tickets=%d err=%v", len(tickets), err)
	}
}

func multipartBody(values map[string]string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for k, v := range values {
		_ = writer.WriteField(k, v)
	}
	_ = writer.Close()
	return body, writer.FormDataContentType()
}
