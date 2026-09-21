package app

import (
	"fmt"
	"os"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/shaiinarab/pabetoop-club/internal/brand"
)

type demoGuardian struct {
	FullName string
	Mobile   string
	Password string
}

// seedDemoData is intentionally opt-in. It creates only deterministic,
// obviously synthetic records for local Docker demos and user-path validation;
// it never runs unless an operator explicitly sets SEED_DEMO=1.
func seedDemoData(app core.App) error {
	if os.Getenv("SEED_DEMO") != "1" {
		return nil
	}
	count, err := app.CountRecords("players")
	if err != nil || count > 0 {
		return err
	}

	now := nowUTC()
	if err := app.RunInTransaction(func(txApp core.App) error {
		manager, err := createDemoManager(txApp)
		if err != nil {
			return err
		}
		guardians, err := createDemoGuardians(txApp)
		if err != nil {
			return err
		}
		plans, err := createDemoPlans(txApp)
		if err != nil {
			return err
		}
		if err := createDemoPlayersAndOperations(txApp, manager, guardians, plans, now); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// The regular scheduler will repeat this safely. Creating the initial queue
	// here makes a fresh test deployment immediately useful to a manager.
	_, err = enqueueDueInvoiceReminders(app, now)
	return err
}

func createDemoManager(app core.App) (*core.Record, error) {
	collection, err := app.FindCollectionByNameOrId("staff")
	if err != nil {
		return nil, err
	}
	manager := core.NewRecord(collection)
	manager.SetEmail("manager@example.test")
	manager.SetPassword("ChangeMe123!")
	manager.Set("display_name", "مدیر نمونه "+brand.Active().NameFA)
	manager.Set("role", "manager")
	return manager, app.Save(manager)
}

func createDemoGuardians(app core.App) ([]*core.Record, error) {
	collection, err := app.FindCollectionByNameOrId("guardians")
	if err != nil {
		return nil, err
	}
	specs := []demoGuardian{
		{FullName: "سرپرست نمونه", Mobile: "09170000001", Password: "ChangeMe123!"},
		{FullName: "مینا احمدی", Mobile: "09170000002", Password: "DemoFamily02!"},
		{FullName: "رضا کاظمی", Mobile: "09170000003", Password: "DemoFamily03!"},
		{FullName: "نسرین موسوی", Mobile: "09170000004", Password: "DemoFamily04!"},
		{FullName: "امیر رضایی", Mobile: "09170000005", Password: "DemoFamily05!"},
		{FullName: "الهام کریمی", Mobile: "09170000006", Password: "DemoFamily06!"},
		{FullName: "سارا حسینی", Mobile: "09170000007", Password: "DemoFamily07!"},
		{FullName: "محمد مرادی", Mobile: "09170000008", Password: "DemoFamily08!"},
		{FullName: "شیوا شریفی", Mobile: "09170000009", Password: "DemoFamily09!"},
		{FullName: "فرهاد نادری", Mobile: "09170000010", Password: "DemoFamily10!"},
		{FullName: "پریا رحیمی", Mobile: "09170000011", Password: "DemoFamily11!"},
		{FullName: "بهنام آقایی", Mobile: "09170000012", Password: "DemoFamily12!"},
	}
	guardians := make([]*core.Record, 0, len(specs))
	for index, spec := range specs {
		guardian := core.NewRecord(collection)
		guardian.SetEmail(fmt.Sprintf("guardian%02d@example.test", index+1))
		guardian.SetPassword(spec.Password)
		guardian.Set("full_name", spec.FullName)
		guardian.Set("mobile", spec.Mobile)
		if err := app.Save(guardian); err != nil {
			return nil, err
		}
		guardians = append(guardians, guardian)
	}
	return guardians, nil
}

func createDemoPlans(app core.App) ([]*core.Record, error) {
	collection, err := app.FindCollectionByNameOrId("plans")
	if err != nil {
		return nil, err
	}
	specs := []planSummary{
		{Name: "نونهالان پایه", AmountRial: 15000000, PeriodMonths: 1, IsActive: true},
		{Name: "نوجوانان پیشرفته", AmountRial: 18000000, PeriodMonths: 1, IsActive: true},
		{Name: "تابستان سه‌ماهه", AmountRial: 45000000, PeriodMonths: 3, IsActive: true},
	}
	plans := make([]*core.Record, 0, len(specs))
	for _, spec := range specs {
		plan := core.NewRecord(collection)
		plan.Set("name", spec.Name)
		plan.Set("amount_rial", spec.AmountRial)
		plan.Set("period_months", spec.PeriodMonths)
		plan.Set("is_active", spec.IsActive)
		if err := app.Save(plan); err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func createDemoPlayersAndOperations(app core.App, manager *core.Record, guardians, plans []*core.Record, now time.Time) error {
	playersCollection, err := app.FindCollectionByNameOrId("players")
	if err != nil {
		return err
	}
	subscriptionsCollection, err := app.FindCollectionByNameOrId("subscriptions")
	if err != nil {
		return err
	}
	linksCollection, err := app.FindCollectionByNameOrId("guardian_players")
	if err != nil {
		return err
	}
	invoicesCollection, err := app.FindCollectionByNameOrId("invoices")
	if err != nil {
		return err
	}
	paymentsCollection, err := app.FindCollectionByNameOrId("payments")
	if err != nil {
		return err
	}

	firstNames := []string{"آرین", "سام", "رادین", "پارسا", "کیان", "بردیا", "آریا", "مانی", "یاسین", "سینا", "هومن", "نوید"}
	lastNames := []string{"آزمایشی", "نمونه", "دمو", "ورزشی", "باشگاهی", "فوتبالی"}
	for number := 1; number <= 180; number++ {
		player := core.NewRecord(playersCollection)
		status := demoPlayerStatus(number)
		player.Set("first_name", firstNames[(number-1)%len(firstNames)])
		player.Set("last_name", lastNames[(number-1)%len(lastNames)])
		player.Set("club_number", fmt.Sprintf("ARA-%03d", number))
		player.Set("birth_date_jalali", fmt.Sprintf("13%02d/%02d/%02d", 88+(number%7), 1+(number%12), 1+(number%27)))
		player.Set("status", status)
		if err := app.Save(player); err != nil {
			return err
		}

		plan := plans[(number-1)%len(plans)]
		subscription := core.NewRecord(subscriptionsCollection)
		subscription.Set("player", player.Id)
		subscription.Set("plan", plan.Id)
		subscription.Set("status", status)
		subscription.Set("updated_by", manager.Id)
		subscription.Set("reason", "اطلاعات آزمایشی استقرار Docker")
		if status == "active" {
			subscription.Set("starts_at", now.AddDate(0, -1, 0).Format(types.DefaultDateLayout))
			subscription.Set("ends_at", now.AddDate(0, 1+(number%2), 0).Format(types.DefaultDateLayout))
		} else if status == "suspended" {
			subscription.Set("starts_at", now.AddDate(0, -2, 0).Format(types.DefaultDateLayout))
			subscription.Set("ends_at", now.AddDate(0, 0, 12).Format(types.DefaultDateLayout))
		} else {
			subscription.Set("starts_at", now.AddDate(0, -3, 0).Format(types.DefaultDateLayout))
			subscription.Set("ends_at", now.AddDate(0, 0, -8).Format(types.DefaultDateLayout))
		}
		if err := app.Save(subscription); err != nil {
			return err
		}

		if number > len(guardians)*2 {
			continue
		}
		guardianIndex := (number - 1) / 2
		guardian := guardians[guardianIndex]
		link := core.NewRecord(linksCollection)
		link.Set("guardian", guardian.Id)
		link.Set("player", player.Id)
		link.Set("relationship", "والد")
		link.Set("is_verified", true)
		if err := app.Save(link); err != nil {
			return err
		}

		invoice := core.NewRecord(invoicesCollection)
		invoice.Set("guardian", guardian.Id)
		invoice.Set("player", player.Id)
		invoice.Set("subscription", subscription.Id)
		invoice.Set("amount_rial", plan.GetInt("amount_rial"))
		invoice.Set("plan_snapshot", plan.GetString("name"))
		invoiceStatus, dueAt := demoInvoiceState(number, now)
		invoice.Set("status", invoiceStatus)
		invoice.Set("due_at", dueAt.Format(types.DefaultDateLayout))
		if err := app.Save(invoice); err != nil {
			return err
		}
		if invoiceStatus == "paid" {
			payment := core.NewRecord(paymentsCollection)
			payment.Set("invoice", invoice.Id)
			payment.Set("provider", "test")
			payment.Set("authority", fmt.Sprintf("DEMO-PAID-%03d", number))
			payment.Set("reference", fmt.Sprintf("TEST-REF-%03d", number))
			payment.Set("amount_rial", plan.GetInt("amount_rial"))
			payment.Set("status", "verified")
			payment.Set("verification_summary", map[string]any{"mode": "synthetic_demo"})
			payment.Set("verified_at", now.AddDate(0, 0, -2).Format(types.DefaultDateLayout))
			if err := app.Save(payment); err != nil {
				return err
			}
		}
	}
	return nil
}

func demoPlayerStatus(number int) string {
	switch {
	case number <= 145:
		return "active"
	case number <= 165:
		return "suspended"
	default:
		return "expired"
	}
}

func demoInvoiceState(number int, now time.Time) (string, time.Time) {
	// The first demo guardian receives one open invoice and one overdue invoice,
	// which makes payment and reminder flows immediately visible after startup.
	if number == 1 {
		return "due", now.AddDate(0, 0, 5)
	}
	if number == 2 {
		return "overdue", now.AddDate(0, 0, -3)
	}
	switch number % 3 {
	case 0:
		return "paid", now.AddDate(0, 0, -10)
	case 1:
		return "due", now.AddDate(0, 0, 4)
	default:
		return "overdue", now.AddDate(0, 0, -2)
	}
}
