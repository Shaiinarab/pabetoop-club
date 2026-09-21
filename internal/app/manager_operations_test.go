package app

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func TestEnrollPlayerInPlanReplacesExistingActiveSubscription(t *testing.T) {
	app := newReminderTestApp(t)
	_, player := createReminderTestGuardianAndPlayer(t, app)
	manager := createOperationsTestManager(t, app)
	firstPlan := createOperationsTestPlan(t, app, "ماهانه", 15000000, 1)
	secondPlan := createOperationsTestPlan(t, app, "فصلی", 42000000, 3)
	start := time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)

	first, err := enrollPlayerInPlan(app, player, firstPlan, manager.Id, start)
	if err != nil {
		t.Fatalf("first enrollment: %v", err)
	}
	if got := first.GetString("status"); got != "active" {
		t.Fatalf("first status = %q; want active", got)
	}

	secondStart := start.AddDate(0, 1, 0)
	second, err := enrollPlayerInPlan(app, player, secondPlan, manager.Id, secondStart)
	if err != nil {
		t.Fatalf("replacement enrollment: %v", err)
	}
	if got := second.GetString("status"); got != "active" {
		t.Fatalf("replacement status = %q; want active", got)
	}

	subscriptions, err := app.FindRecordsByFilter("subscriptions", "player = {:player}", "starts_at", 10, 0, map[string]any{"player": player.Id})
	if err != nil {
		t.Fatalf("find subscriptions: %v", err)
	}
	if len(subscriptions) != 2 {
		t.Fatalf("subscriptions = %d; want 2", len(subscriptions))
	}
	activeCount := 0
	for _, subscription := range subscriptions {
		if subscription.GetString("status") == "active" {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("active subscriptions = %d; want 1", activeCount)
	}
	replaced, err := app.FindRecordById("subscriptions", first.Id)
	if err != nil {
		t.Fatalf("reload replaced subscription: %v", err)
	}
	if got := replaced.GetString("status"); got != "expired" {
		t.Fatalf("replaced subscription status = %q; want expired", got)
	}
}

func TestParseISODate(t *testing.T) {
	fallback := time.Date(2026, time.August, 14, 10, 30, 0, 0, time.UTC)
	parsed, err := parseISODate("2026-09-01", fallback)
	if err != nil || !parsed.Equal(time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("parseISODate valid date = %v, %v", parsed, err)
	}
	if _, err := parseISODate("1405/06/10", fallback); err == nil {
		t.Fatal("Jalali string must not be silently accepted by HTML-date parser")
	}
}

func TestMarkOverdueInvoices(t *testing.T) {
	app := newReminderTestApp(t)
	now := time.Date(2026, time.August, 14, 7, 0, 0, 0, time.UTC)
	guardian, player := createReminderTestGuardianAndPlayer(t, app)
	invoice := createReminderTestInvoice(t, app, guardian.Id, player.Id, "due", now.Add(-time.Minute))

	if err := markOverdueInvoices(app, now); err != nil {
		t.Fatalf("mark overdue: %v", err)
	}
	reloaded, err := app.FindRecordById("invoices", invoice.Id)
	if err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	if got := reloaded.GetString("status"); got != "overdue" {
		t.Fatalf("invoice status = %q; want overdue", got)
	}
}

func createOperationsTestManager(t *testing.T, app core.App) *core.Record {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId("staff")
	if err != nil {
		t.Fatalf("load staff: %v", err)
	}
	manager := core.NewRecord(collection)
	manager.SetEmail("manager.operations@example.test")
	manager.SetPassword("secure-password")
	manager.Set("display_name", "مدیر آزمایشی")
	manager.Set("role", "manager")
	if err := app.Save(manager); err != nil {
		t.Fatalf("create manager: %v", err)
	}
	return manager
}

func createOperationsTestPlan(t *testing.T, app core.App, name string, amount, period int) *core.Record {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId("plans")
	if err != nil {
		t.Fatalf("load plans: %v", err)
	}
	plan := core.NewRecord(collection)
	plan.Set("name", name)
	plan.Set("amount_rial", amount)
	plan.Set("period_months", period)
	plan.Set("is_active", true)
	if err := app.Save(plan); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	return plan
}
