package store

import (
	"path/filepath"
	"testing"
)

func TestStoreEmailTeamReportsAndIdempotency(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	email := "ana@example.com"
	team := "Soporte N2"
	ticket := NewTicket("VPN", "No conecta", "high", "Ana", nil)
	ticket.Email = &email
	ticket.AssignedTeam = &team
	if err := st.Create(ticket); err != nil {
		t.Fatal(err)
	}
	byEmail, err := st.ListByEmail(email)
	if err != nil || len(byEmail) != 1 {
		t.Fatalf("ListByEmail = %d, %v", len(byEmail), err)
	}
	byTeam, err := st.ListByTeam(team)
	if err != nil || len(byTeam) != 1 {
		t.Fatalf("ListByTeam = %d, %v", len(byTeam), err)
	}
	fresh, err := st.MarkProcessed("evt-1")
	if err != nil || !fresh {
		t.Fatalf("first MarkProcessed fresh=%v err=%v", fresh, err)
	}
	fresh, err = st.MarkProcessed("evt-1")
	if err != nil || fresh {
		t.Fatalf("second MarkProcessed fresh=%v err=%v", fresh, err)
	}
	summary, err := st.ReportSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.ByStatus["open"] != 1 || summary.ByPriority["high"] != 1 {
		t.Fatalf("summary mismatch: %+v", summary)
	}
}
