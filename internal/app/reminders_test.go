package app

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func TestEnqueueDueInvoiceRemindersIsIdempotent(t *testing.T) {
	app := newReminderTestApp(t)
	now := time.Date(2026, time.August, 14, 7, 0, 0, 0, time.UTC)
	guardian, player := createReminderTestGuardianAndPlayer(t, app)

	dueInvoice := createReminderTestInvoice(t, app, guardian.Id, player.Id, "due", now.Add(-time.Hour))
	createReminderTestInvoice(t, app, guardian.Id, player.Id, "due", now.Add(time.Hour))
	createReminderTestInvoice(t, app, guardian.Id, player.Id, "paid", now.Add(-time.Hour))

	created, err := enqueueDueInvoiceReminders(app, now)
	if err != nil {
		t.Fatalf("first reminder scan: %v", err)
	}
	if created != 1 {
		t.Fatalf("first reminder scan created %d jobs; want 1", created)
	}

	created, err = enqueueDueInvoiceReminders(app, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("second reminder scan: %v", err)
	}
	if created != 0 {
		t.Fatalf("second reminder scan created %d jobs; want 0", created)
	}

	jobs, err := app.FindRecordsByFilter("reminder_jobs", "invoice = {:invoice}", "", 10, 0, map[string]any{"invoice": dueInvoice.Id})
	if err != nil {
		t.Fatalf("find queued job: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs for due invoice = %d; want 1", len(jobs))
	}
	if got := jobs[0].GetString("status"); got != "queued" {
		t.Fatalf("job status = %q; want queued", got)
	}
}

func newReminderTestApp(t *testing.T) *pocketbase.PocketBase {
	t.Helper()
	app := pocketbase.NewWithConfig(pocketbase.Config{DefaultDataDir: t.TempDir(), HideStartBanner: true})
	if err := app.Bootstrap(); err != nil {
		t.Fatalf("bootstrap PocketBase: %v", err)
	}
	if err := ensureSchoolCollections(app); err != nil {
		t.Fatalf("create Pabetoop collections: %v", err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	return app
}

func createReminderTestGuardianAndPlayer(t *testing.T, app core.App) (*core.Record, *core.Record) {
	t.Helper()
	guardianCollection, err := app.FindCollectionByNameOrId("guardians")
	if err != nil {
		t.Fatalf("load guardians collection: %v", err)
	}
	guardian := core.NewRecord(guardianCollection)
	guardian.SetEmail("09171234567@guardian.pabetoop.local")
	guardian.SetPassword("secure-password")
	guardian.Set("full_name", "ولی آزمایشی")
	guardian.Set("mobile", "09171234567")
	if err := app.Save(guardian); err != nil {
		t.Fatalf("create guardian: %v", err)
	}

	playerCollection, err := app.FindCollectionByNameOrId("players")
	if err != nil {
		t.Fatalf("load players collection: %v", err)
	}
	player := core.NewRecord(playerCollection)
	player.Set("first_name", "بازیکن")
	player.Set("last_name", "آزمایشی")
	player.Set("club_number", "TEST-001")
	player.Set("status", "active")
	if err := app.Save(player); err != nil {
		t.Fatalf("create player: %v", err)
	}
	return guardian, player
}

func createReminderTestInvoice(t *testing.T, app core.App, guardianID, playerID, status string, dueAt time.Time) *core.Record {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId("invoices")
	if err != nil {
		t.Fatalf("load invoices collection: %v", err)
	}
	invoice := core.NewRecord(collection)
	invoice.Set("guardian", guardianID)
	invoice.Set("player", playerID)
	invoice.Set("amount_rial", 15000000)
	invoice.Set("due_at", dueAt.UTC().Format(time.RFC3339))
	invoice.Set("status", status)
	invoice.Set("plan_snapshot", "شهریهٔ آزمایشی")
	if err := app.Save(invoice); err != nil {
		t.Fatalf("create invoice: %v", err)
	}
	return invoice
}
