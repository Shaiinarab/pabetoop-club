// Package app bootstraps Pabetoop Club: a white-label membership,
// subscription and billing platform for sports clubs and academies.
//
// Every customer-facing string comes from internal/brand, so a fork can be
// re-branded with environment variables alone (see .env.example).
package app

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/shaiinarab/pabetoop-club/internal/brand"
)

const schemaHookID = "pabetoop-club-schema"

// Run starts the supported PocketBase 0.39 runtime. The application exposes
// only server-rendered routes; sensitive operations are added behind server
// authorization checks rather than direct browser access to PocketBase data.
func Run() error {
	app := pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir:  "pb_data",
		HideStartBanner: os.Getenv("APP_ENV") == "production",
	})

	app.OnBootstrap().Bind(&hook.Handler[*core.BootstrapEvent]{
		Id: schemaHookID,
		Func: func(e *core.BootstrapEvent) error {
			if err := e.Next(); err != nil {
				return err
			}
			if err := ensureSchoolCollections(e.App); err != nil {
				return err
			}
			return seedDemoData(e.App)
		},
	})

	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Id: "pabetoop-public-routes",
		Func: func(e *core.ServeEvent) error {
			registerPublicRoutes(e)
			if err := registerReminderAutomation(e); err != nil {
				return err
			}
			return e.Next()
		},
	})

	if err := app.Start(); err != nil {
		return fmt.Errorf("start PocketBase: %w", err)
	}
	return nil
}

func registerPublicRoutes(e *core.ServeEvent) {
	e.Router.BindFunc(securityHeaders)
	registerAuthRoutes(e)
	registerDashboardRoutes(e)
	registerManagerBillingRoutes(e)
	registerPaymentRoutes(e)
	e.Router.GET("/sw.js", func(re *core.RequestEvent) error {
		return re.FileFS(os.DirFS("assets"), "js/sw.js")
	})
	e.Router.GET("/assets/{path...}", apis.Static(os.DirFS("assets"), false))
	// More specific than the /assets/{path...} wildcard above, so it wins for this exact path.
	// Register exactly once: net/http's ServeMux panics on a duplicate pattern, and the panic
	// happens while the mux is built — i.e. on the first request, not at build time.
	e.Router.GET("/assets/manifest.webmanifest", manifestHandler)

	e.Router.GET("/_healthz", func(re *core.RequestEvent) error {
		return re.NoContent(http.StatusOK)
	})

	e.Router.GET("/", func(re *core.RequestEvent) error {
		players, _ := re.App.CountRecords("players")
		active, _ := countRecordsByFilter(re.App, "subscriptions", "status = 'active'")
		return re.String(http.StatusOK, publicHomeHTML(players, active))
	})

}

// manifestHandler renders the PWA manifest from the active brand profile.
//
// The manifest used to be a static file, which meant a re-branded deployment
// would still install onto a family's phone under the previous club's name.
// Rendering it per request keeps the installable app and the brand profile in
// sync by construction.
func manifestHandler(re *core.RequestEvent) error {
	profile := brand.Active()
	manifest := map[string]any{
		"id":               "/portal",
		"name":             "پورتال خانواده " + profile.NameFA,
		"short_name":       profile.NameFA,
		"description":      "وضعیت بازیکن و شهریهٔ " + profile.TitleFA(),
		"lang":             "fa",
		"dir":              "rtl",
		"start_url":        "/portal",
		"scope":            "/",
		"display":          "standalone",
		"background_color": "#f7f8f4",
		"theme_color":      "#164b38",
		"icons": []map[string]string{
			{"src": "/assets/icons/icon-192.svg", "sizes": "192x192", "type": "image/svg+xml", "purpose": "any maskable"},
			{"src": "/assets/icons/icon-512.svg", "sizes": "512x512", "type": "image/svg+xml", "purpose": "any maskable"},
		},
	}
	re.Response.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	return re.JSON(http.StatusOK, manifest)
}

// ensureSchoolCollections creates the focused, least-privilege schema. All
// browser writes to operational collections remain disabled; application
// handlers use server-side app methods after explicit authorization.
func ensureSchoolCollections(app core.App) error {
	if err := createCollectionIfMissing(app, staffCollection()); err != nil {
		return err
	}
	if err := createCollectionIfMissing(app, guardiansCollection()); err != nil {
		return err
	}
	if err := createCollectionIfMissing(app, playersCollection()); err != nil {
		return err
	}
	if err := createCollectionIfMissing(app, plansCollection()); err != nil {
		return err
	}

	staff, err := app.FindCollectionByNameOrId("staff")
	if err != nil {
		return err
	}
	guardians, err := app.FindCollectionByNameOrId("guardians")
	if err != nil {
		return err
	}
	players, err := app.FindCollectionByNameOrId("players")
	if err != nil {
		return err
	}
	plans, err := app.FindCollectionByNameOrId("plans")
	if err != nil {
		return err
	}

	if err := createCollectionIfMissing(app, guardianPlayersCollection(guardians.Id, players.Id)); err != nil {
		return err
	}
	if err := createCollectionIfMissing(app, subscriptionsCollection(players.Id, plans.Id, staff.Id)); err != nil {
		return err
	}
	subscriptions, err := app.FindCollectionByNameOrId("subscriptions")
	if err != nil {
		return err
	}
	if err := createCollectionIfMissing(app, invoicesCollection(guardians.Id, players.Id, subscriptions.Id)); err != nil {
		return err
	}
	invoices, err := app.FindCollectionByNameOrId("invoices")
	if err != nil {
		return err
	}
	if err := createCollectionIfMissing(app, paymentsCollection(invoices.Id)); err != nil {
		return err
	}
	if err := createCollectionIfMissing(app, reminderJobsCollection(invoices.Id, guardians.Id)); err != nil {
		return err
	}
	if err := createCollectionIfMissing(app, auditLogsCollection(staff.Id)); err != nil {
		return err
	}

	return app.ReloadCachedCollections()
}

func countRecordsByFilter(app core.App, collection, filter string) (int64, error) {
	records, err := app.FindRecordsByFilter(collection, filter, "", 1000, 0)
	if err != nil {
		return 0, err
	}
	return int64(len(records)), nil
}

func createCollectionIfMissing(app core.App, collection *core.Collection) error {
	if _, err := app.FindCollectionByNameOrId(collection.Name); err == nil {
		return nil
	}
	if err := app.Save(collection); err != nil {
		return fmt.Errorf("create collection %s: %w", collection.Name, err)
	}
	return nil
}

func staffCollection() *core.Collection {
	collection := core.NewAuthCollection("staff")
	collection.Fields.Add(
		&core.TextField{Name: "display_name", Required: true, Max: 100},
		&core.TextField{Name: "role", Required: true, Max: 20},
	)
	collection.AddIndex("idx_staff_role", false, "role", "")
	return collection
}

func guardiansCollection() *core.Collection {
	collection := core.NewAuthCollection("guardians")
	collection.Fields.Add(
		&core.TextField{Name: "full_name", Required: true, Max: 140},
		&core.TextField{Name: "mobile", Required: true, Max: 16},
	)
	collection.AddIndex("idx_guardians_mobile", true, "mobile", "")
	return collection
}

func playersCollection() *core.Collection {
	collection := core.NewBaseCollection("players")
	collection.Fields.Add(
		&core.TextField{Name: "first_name", Required: true, Max: 80},
		&core.TextField{Name: "last_name", Required: true, Max: 80},
		&core.TextField{Name: "club_number", Required: true, Max: 32},
		&core.TextField{Name: "birth_date_jalali", Max: 10},
		&core.TextField{Name: "status", Required: true, Max: 20},
		&core.TextField{Name: "internal_note", Max: 1000, Hidden: true},
	)
	collection.AddIndex("idx_players_club_number", true, "club_number", "")
	collection.AddIndex("idx_players_status", false, "status", "")
	return collection
}

func plansCollection() *core.Collection {
	collection := core.NewBaseCollection("plans")
	collection.Fields.Add(
		&core.TextField{Name: "name", Required: true, Max: 100},
		&core.NumberField{Name: "amount_rial", Required: true, Min: floatPtr(1)},
		&core.NumberField{Name: "period_months", Required: true, Min: floatPtr(1)},
		&core.BoolField{Name: "is_active", Required: true},
	)
	return collection
}

func guardianPlayersCollection(guardianID, playerID string) *core.Collection {
	collection := core.NewBaseCollection("guardian_players")
	collection.Fields.Add(
		&core.RelationField{Name: "guardian", CollectionId: guardianID, Required: true, MaxSelect: 1},
		&core.RelationField{Name: "player", CollectionId: playerID, Required: true, MaxSelect: 1},
		&core.TextField{Name: "relationship", Required: true, Max: 40},
		&core.BoolField{Name: "is_verified", Required: true},
	)
	collection.AddIndex("idx_guardian_player", true, "guardian,player", "")
	return collection
}

func subscriptionsCollection(playerID, planID, staffID string) *core.Collection {
	collection := core.NewBaseCollection("subscriptions")
	collection.Fields.Add(
		&core.RelationField{Name: "player", CollectionId: playerID, Required: true, MaxSelect: 1},
		&core.RelationField{Name: "plan", CollectionId: planID, Required: true, MaxSelect: 1},
		&core.TextField{Name: "status", Required: true, Max: 20},
		&core.DateField{Name: "starts_at", Required: true},
		&core.DateField{Name: "ends_at", Required: true},
		&core.TextField{Name: "reason", Max: 400},
		&core.RelationField{Name: "updated_by", CollectionId: staffID, MaxSelect: 1},
	)
	collection.AddIndex("idx_subscriptions_player", false, "player", "")
	collection.AddIndex("idx_subscriptions_status", false, "status", "")
	return collection
}

func invoicesCollection(guardianID, playerID, subscriptionID string) *core.Collection {
	collection := core.NewBaseCollection("invoices")
	collection.Fields.Add(
		&core.RelationField{Name: "guardian", CollectionId: guardianID, Required: true, MaxSelect: 1},
		&core.RelationField{Name: "player", CollectionId: playerID, Required: true, MaxSelect: 1},
		&core.RelationField{Name: "subscription", CollectionId: subscriptionID, MaxSelect: 1},
		&core.NumberField{Name: "amount_rial", Required: true, Min: floatPtr(1)},
		&core.DateField{Name: "due_at", Required: true},
		&core.TextField{Name: "status", Required: true, Max: 20},
		&core.TextField{Name: "plan_snapshot", Max: 200},
	)
	collection.AddIndex("idx_invoices_guardian", false, "guardian", "")
	collection.AddIndex("idx_invoices_status", false, "status", "")
	return collection
}

func reminderJobsCollection(invoiceID, guardianID string) *core.Collection {
	collection := core.NewBaseCollection("reminder_jobs")
	collection.Fields.Add(
		&core.RelationField{Name: "invoice", CollectionId: invoiceID, Required: true, MaxSelect: 1},
		&core.RelationField{Name: "guardian", CollectionId: guardianID, Required: true, MaxSelect: 1},
		&core.TextField{Name: "status", Required: true, Max: 20},
		&core.TextField{Name: "channel", Required: true, Max: 40},
		&core.TextField{Name: "reminder_key", Required: true, Max: 80},
		&core.DateField{Name: "scheduled_for", Required: true},
		&core.DateField{Name: "completed_at"},
		&core.NumberField{Name: "attempt_count", Min: floatPtr(0)},
	)
	collection.AddIndex("idx_reminder_invoice_key", true, "invoice,reminder_key", "")
	collection.AddIndex("idx_reminder_status_schedule", false, "status,scheduled_for", "")
	return collection
}

func paymentsCollection(invoiceID string) *core.Collection {
	collection := core.NewBaseCollection("payments")
	collection.Fields.Add(
		&core.RelationField{Name: "invoice", CollectionId: invoiceID, Required: true, MaxSelect: 1},
		&core.TextField{Name: "provider", Required: true, Max: 30},
		&core.TextField{Name: "authority", Required: true, Max: 120},
		&core.TextField{Name: "reference", Max: 120},
		&core.NumberField{Name: "amount_rial", Required: true, Min: floatPtr(1)},
		&core.TextField{Name: "status", Required: true, Max: 20},
		&core.JSONField{Name: "verification_summary", Hidden: true},
		&core.DateField{Name: "verified_at"},
	)
	collection.AddIndex("idx_payments_authority", true, "provider,authority", "")
	collection.AddIndex("idx_payments_invoice", false, "invoice", "")
	return collection
}

func auditLogsCollection(staffID string) *core.Collection {
	collection := core.NewBaseCollection("audit_logs")
	collection.Fields.Add(
		&core.RelationField{Name: "actor", CollectionId: staffID, MaxSelect: 1},
		&core.TextField{Name: "action", Required: true, Max: 100},
		&core.TextField{Name: "target_type", Required: true, Max: 50},
		&core.TextField{Name: "target_id", Required: true, Max: 40},
		&core.JSONField{Name: "metadata"},
	)
	collection.AddIndex("idx_audit_target", false, "target_type,target_id", "")
	return collection
}

func floatPtr(value float64) *float64 { return &value }

func publicHomeHTML(playerCount, activeSubscriptions int64) string {
	return fmt.Sprintf(`<!doctype html><html lang="fa" dir="rtl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title><style>body{margin:0;font-family:Tahoma,sans-serif;background:#f5f7f3;color:#173325}.hero{padding:5rem 8vw;background:#103f2f;color:#fff}.tag{color:#f2c94c}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(170px,1fr));gap:1rem;padding:2rem 8vw}.card{background:#fff;border-radius:16px;padding:1.25rem;box-shadow:0 8px 24px #17332514}.cta{display:inline-block;background:#f2c94c;color:#173325;padding:.8rem 1.1rem;border-radius:10px;text-decoration:none;font-weight:bold}</style></head><body><main><section class="hero"><p class="tag">%s</p><h1>%s</h1><p>مدیریت روشن شهریه و وضعیت بازیکن برای خانواده‌ها؛ یک میزکار ساده برای مدیر باشگاه.</p><a class="cta" href="/manager">ورود مدیر</a></section><section class="grid"><article class="card"><strong>بازیکن ثبت‌شده</strong><p>%d</p></article><article class="card"><strong>اشتراک فعال</strong><p>%d</p></article><article class="card"><strong>پرداخت امن</strong><p>محیط آزمایشی</p></article></section></main></body></html>`,
		brand.Active().TitleFA(), brand.Active().HeroTagFA(), brand.Active().TitleFA(),
		playerCount, activeSubscriptions)
}

func managerOverviewHTML(players, guardians, dueInvoices int64) string {
	return fmt.Sprintf(`<!doctype html><html lang="fa" dir="rtl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title></head><body><h1>%s</h1><p>بازیکنان: %d | سرپرستان: %d | صورتحساب‌های سررسیدشده: %d</p><p>نسخهٔ عملیاتی داشبورد در حال تکمیل است.</p></body></html>`,
		brand.Active().ManagerTitleFA(), brand.Active().ManagerHeadingFA(),
		players, guardians, dueInvoices)
}

// nowUTC is retained for future invoice and subscription helpers and ensures
// all stored business timestamps are UTC based.
func nowUTC() time.Time { return time.Now().UTC() }
