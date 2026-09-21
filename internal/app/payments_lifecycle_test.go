package app

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func TestSettlePaymentActivatesSubscriptionAndPlayer(t *testing.T) {
	app := newReminderTestApp(t)
	guardian, player := createReminderTestGuardianAndPlayer(t, app)
	manager := createOperationsTestManager(t, app)
	plan := createOperationsTestPlan(t, app, "ماهانه", 15000000, 1)
	subscription, err := enrollPlayerInPlan(app, player, plan, manager.Id, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("enroll player: %v", err)
	}
	subscription.Set("status", "expired")
	if err := app.Save(subscription); err != nil {
		t.Fatalf("expire subscription: %v", err)
	}
	player.Set("status", "expired")
	if err := app.Save(player); err != nil {
		t.Fatalf("expire player: %v", err)
	}
	invoice := createReminderTestInvoice(t, app, guardian.Id, player.Id, "due", time.Now().UTC())
	invoice.Set("subscription", subscription.Id)
	if err := app.Save(invoice); err != nil {
		t.Fatalf("link invoice subscription: %v", err)
	}
	payment := createLifecycleTestPayment(t, app, invoice.Id)

	if err := settlePayment(app, payment, invoice, paymentVerifyResponse{Reference: "TEST-REF-001", Summary: map[string]any{"mode": "test"}}); err != nil {
		t.Fatalf("settle payment: %v", err)
	}
	assertLifecycleStatuses(t, app, invoice.Id, subscription.Id, player.Id, "paid", "active", "active")
}

func TestSettlePaymentKeepsManagerSuspension(t *testing.T) {
	app := newReminderTestApp(t)
	guardian, player := createReminderTestGuardianAndPlayer(t, app)
	manager := createOperationsTestManager(t, app)
	plan := createOperationsTestPlan(t, app, "ماهانه", 15000000, 1)
	subscription, err := enrollPlayerInPlan(app, player, plan, manager.Id, time.Now().UTC())
	if err != nil {
		t.Fatalf("enroll player: %v", err)
	}
	player.Set("status", "suspended")
	if err := app.Save(player); err != nil {
		t.Fatalf("suspend player: %v", err)
	}
	invoice := createReminderTestInvoice(t, app, guardian.Id, player.Id, "due", time.Now().UTC())
	invoice.Set("subscription", subscription.Id)
	if err := app.Save(invoice); err != nil {
		t.Fatalf("link invoice subscription: %v", err)
	}
	payment := createLifecycleTestPayment(t, app, invoice.Id)

	if err := settlePayment(app, payment, invoice, paymentVerifyResponse{Reference: "TEST-REF-002", Summary: map[string]any{"mode": "test"}}); err != nil {
		t.Fatalf("settle payment: %v", err)
	}
	assertLifecycleStatuses(t, app, invoice.Id, subscription.Id, player.Id, "paid", "active", "suspended")
}

func createLifecycleTestPayment(t *testing.T, app core.App, invoiceID string) *core.Record {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId("payments")
	if err != nil {
		t.Fatalf("load payments: %v", err)
	}
	payment := core.NewRecord(collection)
	payment.Set("invoice", invoiceID)
	payment.Set("provider", "test")
	payment.Set("authority", "TEST-AUTHORITY")
	payment.Set("amount_rial", 15000000)
	payment.Set("status", "initiated")
	if err := app.Save(payment); err != nil {
		t.Fatalf("create payment: %v", err)
	}
	return payment
}

func assertLifecycleStatuses(t *testing.T, app core.App, invoiceID, subscriptionID, playerID, invoiceStatus, subscriptionStatus, playerStatus string) {
	t.Helper()
	invoice, err := app.FindRecordById("invoices", invoiceID)
	if err != nil {
		t.Fatalf("reload invoice: %v", err)
	}
	subscription, err := app.FindRecordById("subscriptions", subscriptionID)
	if err != nil {
		t.Fatalf("reload subscription: %v", err)
	}
	player, err := app.FindRecordById("players", playerID)
	if err != nil {
		t.Fatalf("reload player: %v", err)
	}
	if got := invoice.GetString("status"); got != invoiceStatus {
		t.Fatalf("invoice status = %q; want %q", got, invoiceStatus)
	}
	if got := subscription.GetString("status"); got != subscriptionStatus {
		t.Fatalf("subscription status = %q; want %q", got, subscriptionStatus)
	}
	if got := player.GetString("status"); got != playerStatus {
		t.Fatalf("player status = %q; want %q", got, playerStatus)
	}
}
