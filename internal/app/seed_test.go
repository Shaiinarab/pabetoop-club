package app

import "testing"

func TestSeedDemoDataCreatesRealisticSyntheticOperations(t *testing.T) {
	t.Setenv("SEED_DEMO", "1")
	app := newReminderTestApp(t)

	if err := seedDemoData(app); err != nil {
		t.Fatalf("seed demo data: %v", err)
	}
	players, err := app.CountRecords("players")
	if err != nil {
		t.Fatalf("count players: %v", err)
	}
	if players != 180 {
		t.Fatalf("players = %d; want 180", players)
	}
	guardians, err := app.CountRecords("guardians")
	if err != nil {
		t.Fatalf("count guardians: %v", err)
	}
	if guardians != 12 {
		t.Fatalf("guardians = %d; want 12", guardians)
	}
	plans, err := app.CountRecords("plans")
	if err != nil {
		t.Fatalf("count plans: %v", err)
	}
	if plans != 3 {
		t.Fatalf("plans = %d; want 3", plans)
	}
	overdue, err := countRecordsByFilter(app, "invoices", "status = 'overdue'")
	if err != nil {
		t.Fatalf("count overdue invoices: %v", err)
	}
	if overdue == 0 {
		t.Fatal("demo seed must include overdue invoices")
	}
	paid, err := countRecordsByFilter(app, "invoices", "status = 'paid'")
	if err != nil {
		t.Fatalf("count paid invoices: %v", err)
	}
	if paid == 0 {
		t.Fatal("demo seed must include paid invoices")
	}
	reminders, err := countRecordsByFilter(app, "reminder_jobs", "status = 'queued'")
	if err != nil {
		t.Fatalf("count reminder jobs: %v", err)
	}
	if reminders == 0 {
		t.Fatal("demo seed must create actionable reminder work")
	}

	if err := seedDemoData(app); err != nil {
		t.Fatalf("repeat seed should be a no-op: %v", err)
	}
	playersAfterRepeat, err := app.CountRecords("players")
	if err != nil {
		t.Fatalf("count players after repeat: %v", err)
	}
	if playersAfterRepeat != 180 {
		t.Fatalf("repeat seed players = %d; want 180", playersAfterRepeat)
	}
}
