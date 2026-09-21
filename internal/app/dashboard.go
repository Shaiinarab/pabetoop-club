package app

import (
	"fmt"
	"html"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/shaiinarab/pabetoop-club/internal/brand"
	ptime "github.com/yaa110/go-persian-calendar"
)

type playerSummary struct {
	ID         string
	FirstName  string
	LastName   string
	ClubNumber string
	Status     string
	BirthDate  string
}

func registerDashboardRoutes(e *core.ServeEvent) {
	e.Router.GET("/manager", managerDashboard)
	e.Router.GET("/portal", guardianPortal)
	e.Router.GET("/manager/players", managerPlayersFragment)
	e.Router.POST("/manager/players", createPlayer)
	e.Router.GET("/manager/players/{id}", managerPlayerDetail)
}

func guardianPortal(re *core.RequestEvent) error {
	session, err := currentSession(re.App, re.Request)
	if err != nil {
		return re.Redirect(http.StatusSeeOther, "/login")
	}
	if session.Collection().Name != "guardians" {
		return re.ForbiddenError("guardian access required", nil)
	}
	csrf := csrfToken(re)

	links, err := re.App.FindRecordsByFilter(
		"guardian_players",
		"guardian = {:guardian} && is_verified = true",
		"",
		25,
		0,
		map[string]any{"guardian": session.Id},
	)
	if err != nil {
		return re.InternalServerError("بارگذاری پرونده‌های بازیکن ناموفق بود.", err)
	}

	cards := make([]string, 0, len(links))
	for _, link := range links {
		player, err := re.App.FindRecordById("players", link.GetString("player"))
		if err != nil {
			continue
		}
		subscription, _ := latestSubscription(re.App, player.Id)
		invoice, _ := latestInvoice(re.App, session.Id, player.Id)
		cards = append(cards, guardianPlayerCard(player, subscription, invoice))
	}
	if len(cards) == 0 {
		cards = append(cards, `<div class="empty"><strong>هنوز بازیکنی به حساب شما متصل نشده است.</strong><p>برای اتصال پرونده با مدیر باشگاه تماس بگیرید.</p></div>`)
	}

	return re.HTML(http.StatusOK, pageShell("پورتال خانواده", session.GetString("full_name"), "portal", `<section class="page-heading"><div><p class="eyebrow">فضای خانواده</p><h1>وضعیت فرزند شما</h1><p>اشتراک، صورتحساب و مسیر پرداخت در یک نمای شفاف.</p></div><form method="post" action="/logout">`+hiddenCSRF(csrf)+`<button class="button button-quiet">خروج</button></form></section><section class="player-portal-grid">`+strings.Join(cards, "")+`</section>`))
}

func managerPlayersFragment(re *core.RequestEvent) error {
	if _, err := requireManager(re); err != nil {
		return err
	}
	query := strings.TrimSpace(re.Request.URL.Query().Get("q"))
	players, err := findPlayers(re.App, query)
	if err != nil {
		return re.InternalServerError("جست‌وجوی بازیکنان ناموفق بود.", err)
	}
	return re.HTML(http.StatusOK, playerTableRows(players))
}

func createPlayer(re *core.RequestEvent) error {
	manager, err := requireManager(re)
	if err != nil {
		return err
	}
	if err := validateCSRF(re); err != nil {
		return err
	}
	firstName := compactText(re.Request.FormValue("first_name"))
	lastName := compactText(re.Request.FormValue("last_name"))
	clubNumber := strings.ToUpper(compactText(re.Request.FormValue("club_number")))
	birthDate := compactText(re.Request.FormValue("birth_date_jalali"))
	status := normalizePlayerStatus(re.Request.FormValue("status"))
	if firstName == "" || lastName == "" || clubNumber == "" {
		return re.BadRequestError("نام، نام خانوادگی و شماره باشگاهی الزامی است.", nil)
	}

	collection, err := re.App.FindCollectionByNameOrId("players")
	if err != nil {
		return re.InternalServerError("مجموعهٔ بازیکنان در دسترس نیست.", err)
	}
	player := core.NewRecord(collection)
	player.Set("first_name", firstName)
	player.Set("last_name", lastName)
	player.Set("club_number", clubNumber)
	player.Set("birth_date_jalali", birthDate)
	player.Set("status", status)
	if err := re.App.Save(player); err != nil {
		return re.BadRequestError("ثبت بازیکن ناموفق بود؛ شمارهٔ باشگاهی باید یکتا باشد.", nil)
	}
	_ = writeAudit(re.App, manager.Id, "player.created", "player", player.Id, map[string]any{"club_number": clubNumber})

	if re.Request.Header.Get("HX-Request") == "true" {
		return re.HTML(http.StatusCreated, playerTableRows([]playerSummary{summaryFromRecord(player)}))
	}
	return re.Redirect(http.StatusSeeOther, "/manager")
}

func managerPlayerDetail(re *core.RequestEvent) error {
	if _, err := requireManager(re); err != nil {
		return err
	}
	player, err := re.App.FindRecordById("players", re.Request.PathValue("id"))
	if err != nil {
		return re.NotFoundError("بازیکن پیدا نشد.", err)
	}
	subscription, _ := latestSubscription(re.App, player.Id)
	csrf := csrfToken(re)
	return re.HTML(http.StatusOK, `<section class="drawer-detail"><p class="eyebrow">پروندهٔ بازیکن</p><h2>`+playerName(player)+`</h2><dl><div><dt>شماره باشگاهی</dt><dd>`+escape(player.GetString("club_number"))+`</dd></div><div><dt>تاریخ تولد</dt><dd>`+escape(orDash(player.GetString("birth_date_jalali")))+`</dd></div><div><dt>وضعیت</dt><dd><span class="status status-`+escape(player.GetString("status"))+`">`+statusLabel(player.GetString("status"))+`</span></dd></div><div><dt>پایان اشتراک</dt><dd>`+subscriptionEndLabel(subscription)+`</dd></div></dl><form class="inline-form" method="post" action="/manager/players/`+escape(player.Id)+`/status">`+hiddenCSRF(csrf)+`<label>وضعیت پرونده<select name="status"><option value="active">فعال</option><option value="suspended">تعلیق</option><option value="expired">منقضی</option></select></label><button class="button button-small" type="submit">به‌روزرسانی وضعیت</button></form></section>`)
}

func requireManager(re *core.RequestEvent) (*core.Record, error) {
	session, err := currentSession(re.App, re.Request)
	if err != nil {
		return nil, re.UnauthorizedError("ورود مدیر لازم است.", nil)
	}
	if !isManager(session) {
		return nil, re.ForbiddenError("دسترسی مدیر لازم است.", nil)
	}
	return session, nil
}

func findPlayers(app core.App, query string) ([]playerSummary, error) {
	records, err := app.FindRecordsByFilter("players", "", "", 500, 0)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(query)
	result := make([]playerSummary, 0, len(records))
	for _, record := range records {
		summary := summaryFromRecord(record)
		haystack := strings.ToLower(summary.FirstName + " " + summary.LastName + " " + summary.ClubNumber)
		if needle == "" || strings.Contains(haystack, needle) {
			result = append(result, summary)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ClubNumber < result[j].ClubNumber })
	return result, nil
}

func summaryFromRecord(record *core.Record) playerSummary {
	return playerSummary{ID: record.Id, FirstName: record.GetString("first_name"), LastName: record.GetString("last_name"), ClubNumber: record.GetString("club_number"), Status: record.GetString("status"), BirthDate: record.GetString("birth_date_jalali")}
}

func playerTableRows(players []playerSummary) string {
	if len(players) == 0 {
		return `<tr><td colspan="5"><div class="empty compact">بازیکنی مطابق جست‌وجو پیدا نشد.</div></td></tr>`
	}
	var rows strings.Builder
	for _, player := range players {
		rows.WriteString(`<tr><td><strong>` + escape(player.FirstName+" "+player.LastName) + `</strong><small>` + escape(orDash(player.BirthDate)) + `</small></td><td><code>` + escape(player.ClubNumber) + `</code></td><td><span class="status status-` + escape(player.Status) + `">` + statusLabel(player.Status) + `</span></td><td><button class="table-action" hx-get="/manager/players/` + escape(player.ID) + `" hx-target="#player-detail" hx-swap="innerHTML">مشاهده</button></td></tr>`)
	}
	return rows.String()
}

func latestSubscription(app core.App, playerID string) (*core.Record, error) {
	records, err := app.FindRecordsByFilter("subscriptions", "player = {:player}", "-ends_at", 1, 0, map[string]any{"player": playerID})
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return records[0], nil
}

func latestInvoice(app core.App, guardianID, playerID string) (*core.Record, error) {
	records, err := app.FindRecordsByFilter("invoices", "guardian = {:guardian} && player = {:player}", "-due_at", 1, 0, map[string]any{"guardian": guardianID, "player": playerID})
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return records[0], nil
}

func guardianPlayerCard(player, subscription, invoice *core.Record) string {
	subscriptionStatus := "بدون اشتراک"
	subscriptionClass := "suspended"
	if subscription != nil {
		subscriptionStatus = statusLabel(subscription.GetString("status"))
		subscriptionClass = subscription.GetString("status")
	}
	invoiceBlock := `<p class="muted">در حال حاضر صورتحساب جدیدی ندارید.</p>`
	if invoice != nil {
		if invoice.GetString("status") == "paid" {
			invoiceBlock = `<div class="invoice-line"><div><span>آخرین صورتحساب</span><strong>` + formatRial(invoice.GetInt("amount_rial")) + ` ریال</strong></div><span class="status status-paid">پرداخت‌شده</span></div>`
		} else {
			invoiceBlock = `<div class="invoice-line"><div><span>صورتحساب</span><strong>` + formatRial(invoice.GetInt("amount_rial")) + ` ریال</strong></div><a class="button button-small" href="/pay/` + escape(invoice.Id) + `">پرداخت</a></div>`
		}
	}
	return `<article class="portal-card"><div class="card-top"><div><p class="eyebrow">بازیکن</p><h2>` + playerName(player) + `</h2><code>` + escape(player.GetString("club_number")) + `</code></div><span class="status status-` + escape(subscriptionClass) + `">` + subscriptionStatus + `</span></div><div class="subscription-meta"><div><span>وضعیت پرونده</span><strong>` + statusLabel(player.GetString("status")) + `</strong></div><div><span>پایان اشتراک</span><strong>` + subscriptionEndLabel(subscription) + `</strong></div></div>` + invoiceBlock + `</article>`
}

func writeAudit(app core.App, actor, action, targetType, targetID string, metadata map[string]any) error {
	collection, err := app.FindCollectionByNameOrId("audit_logs")
	if err != nil {
		return err
	}
	record := core.NewRecord(collection)
	record.Set("actor", actor)
	record.Set("action", action)
	record.Set("target_type", targetType)
	record.Set("target_id", targetID)
	record.Set("metadata", metadata)
	return app.Save(record)
}

func pageShell(title, userName, area, body string) string {
	return `<!doctype html><html lang="fa" dir="rtl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light"><meta name="theme-color" content="#164b38"><link rel="manifest" href="/assets/manifest.webmanifest"><link rel="icon" href="/assets/icons/icon-192.svg" type="image/svg+xml"><link rel="apple-touch-icon" href="/assets/icons/icon-192.svg"><title>` + escape(title) + ` | ` + escape(brand.Active().NameFA) + `</title><script defer src="/assets/js/htmx-4.0.0-beta6.min.js"></script><script defer src="/assets/js/pwa-register.js"></script><style>` + appCSS + `</style></head><body data-area="` + escape(area) + `"><div class="app-shell"><header class="app-header"><a href="/" class="brand"><span>` + escape(brand.Active().ShortFA) + `</span><strong>` + escape(brand.Active().NameFA) + `</strong></a><nav><a href="/portal">پورتال خانواده</a><a href="/manager">مدیریت</a></nav><div class="identity">` + escape(orDash(userName)) + `</div></header><main class="main-content">` + body + `</main></div></body></html>`
}

const appCSS = `:root{--ink:#112b22;--pine:#164b38;--leaf:#2c7559;--sun:#f2c94c;--paper:#f7f8f4;--line:#dce5de;--muted:#66766d;--danger:#af3e3e}*{box-sizing:border-box}body{margin:0;background:var(--paper);color:var(--ink);font-family:Tahoma,"Vazirmatn",sans-serif}.app-shell{min-height:100vh}.app-header{height:72px;display:flex;align-items:center;gap:2rem;padding:0 6vw;background:#fff;border-bottom:1px solid var(--line);position:sticky;top:0;z-index:5}.brand{display:inline-flex;gap:.55rem;align-items:center;color:var(--pine);text-decoration:none;font-size:1.1rem}.brand span{display:grid;place-items:center;width:32px;height:32px;border-radius:9px;color:white;background:var(--pine);font-weight:900}.app-header nav{display:flex;gap:1rem}.app-header nav a{color:var(--muted);font-size:.9rem;text-decoration:none}.identity{margin-inline-start:auto;font-size:.85rem;color:var(--muted)}.main-content{max-width:1280px;margin:0 auto;padding:3rem 6vw}.page-heading{display:flex;justify-content:space-between;gap:1.25rem;align-items:flex-start;margin-bottom:2rem}.eyebrow{margin:0 0 .4rem;color:var(--leaf);font-size:.78rem;font-weight:bold}.page-heading h1{font-size:clamp(1.7rem,3vw,2.5rem);margin:0 0 .6rem}.page-heading p:not(.eyebrow){margin:0;color:var(--muted)}.button{display:inline-flex;justify-content:center;align-items:center;border:0;border-radius:10px;background:var(--pine);color:#fff;padding:.72rem 1rem;font:inherit;font-weight:bold;text-decoration:none;cursor:pointer}.button-small{font-size:.85rem;padding:.58rem .85rem}.button-quiet{background:#edf2ed;color:var(--pine)}.stats-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:1rem;margin-bottom:1.5rem}.stat{background:white;border:1px solid var(--line);border-radius:16px;padding:1.2rem}.stat span{display:block;color:var(--muted);font-size:.85rem}.stat strong{display:block;font-size:2rem;margin-top:.4rem}.manager-grid{display:grid;grid-template-columns:minmax(0,2.15fr) minmax(280px,.85fr);gap:1.25rem}.panel,.portal-card{background:white;border:1px solid var(--line);border-radius:18px;padding:1.25rem;box-shadow:0 8px 28px #164b3809}.panel h2{font-size:1.05rem;margin:0 0 1rem}.search{width:100%;padding:.75rem 1rem;border:1px solid var(--line);border-radius:10px;background:#fbfcfa;font:inherit;margin-bottom:1rem}table{width:100%;border-collapse:collapse}th{text-align:right;color:var(--muted);font-size:.78rem;padding:.8rem;border-bottom:1px solid var(--line)}td{padding:.85rem .8rem;border-bottom:1px solid #edf0ed;font-size:.9rem}td small{display:block;color:var(--muted);margin-top:.25rem}code{font-family:ui-monospace,monospace;color:var(--leaf);background:#eff6f1;padding:.2rem .35rem;border-radius:5px}.table-action{background:transparent;border:0;color:var(--pine);font:inherit;cursor:pointer}.status{display:inline-flex;padding:.25rem .52rem;border-radius:999px;font-size:.75rem;font-weight:bold}.status-active{background:#e7f5ea;color:#27743a}.status-suspended{background:#fff1df;color:#9d6712}.status-expired,.status-overdue{background:#fceaea;color:#a13232}.status-paid{background:#e4f2f7;color:#1e627e}.field{display:grid;gap:.4rem;margin:.7rem 0}.field label{font-size:.8rem;color:var(--muted)}.field input,.field select{border:1px solid var(--line);border-radius:9px;padding:.7rem;font:inherit;background:#fff}.player-portal-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(300px,1fr));gap:1.25rem}.card-top{display:flex;justify-content:space-between;gap:.75rem}.card-top h2{margin:.25rem 0 .5rem;font-size:1.3rem}.subscription-meta{display:grid;grid-template-columns:repeat(2,1fr);gap:.75rem;margin:1.35rem 0}.subscription-meta div{background:#f6f8f6;border-radius:10px;padding:.75rem}.subscription-meta span{display:block;color:var(--muted);font-size:.75rem;margin-bottom:.35rem}.invoice-line{display:flex;align-items:center;justify-content:space-between;gap:1rem;border-top:1px solid var(--line);padding-top:1rem}.invoice-line span{display:block;color:var(--muted);font-size:.78rem}.empty{padding:1.3rem;background:#fbfcfa;border:1px dashed #c9d6cb;border-radius:12px;color:var(--muted)}.empty.compact{padding:.75rem}.plan-list{display:grid;gap:.55rem;list-style:none;padding:0;margin:1rem 0 0}.plan-list li{padding:.65rem .75rem;background:#f8faf8;border:1px solid #edf0ed;border-radius:10px}.plan-list strong,.plan-list span{display:block}.plan-list span{margin-top:.22rem;color:var(--muted);font-size:.75rem}.inline-form{display:flex;align-items:end;gap:.6rem;flex-wrap:wrap;margin-top:1rem}.inline-form label{display:grid;gap:.35rem;color:var(--muted);font-size:.78rem}.inline-form select{border:1px solid var(--line);border-radius:8px;padding:.45rem;background:#fff;font:inherit}.reminder-panel .muted{margin:-.35rem 0 1rem}.reminder-list{display:grid;gap:.7rem;list-style:none;padding:0;margin:0}.reminder-list li{display:flex;align-items:center;justify-content:space-between;gap:.75rem;padding:.75rem;background:#f8faf8;border:1px solid #edf0ed;border-radius:10px}.reminder-list strong,.reminder-list span{display:block}.reminder-list span{margin-top:.22rem;color:var(--muted);font-size:.75rem}.drawer-detail dl{display:grid;grid-template-columns:repeat(2,1fr);gap:.8rem}.drawer-detail dl div{padding:.8rem;background:#f7f9f7;border-radius:10px}.drawer-detail dt{font-size:.75rem;color:var(--muted);margin-bottom:.35rem}.drawer-detail dd{margin:0;font-weight:bold}@media(max-width:860px){.app-header{gap:1rem;padding:0 1rem}.app-header nav{display:none}.main-content{padding:1.5rem 1rem}.stats-grid{grid-template-columns:1fr}.manager-grid{grid-template-columns:1fr}.page-heading{flex-direction:column}.identity{margin-inline-start:0}.drawer-detail dl{grid-template-columns:1fr}}`

func playerName(player *core.Record) string {
	return escape(strings.TrimSpace(player.GetString("first_name") + " " + player.GetString("last_name")))
}
func escape(value string) string { return html.EscapeString(value) }
func orDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func compactText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}
func normalizePlayerStatus(value string) string {
	switch value {
	case "active", "suspended", "expired":
		return value
	default:
		return "active"
	}
}
func statusLabel(value string) string {
	switch value {
	case "active":
		return "فعال"
	case "suspended":
		return "تعلیق"
	case "expired":
		return "منقضی"
	case "due":
		return "سررسید"
	case "overdue":
		return "معوق"
	case "paid":
		return "پرداخت‌شده"
	case "initiated":
		return "در انتظار تأیید"
	default:
		return "—"
	}
}
func subscriptionEndLabel(subscription *core.Record) string {
	if subscription == nil || subscription.GetString("ends_at") == "" {
		return "—"
	}
	value := subscription.GetString("ends_at")
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05.000Z"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return ptime.New(parsed).Format("yyyy/MM/dd")
		}
	}
	return escape(value)
}
func formatRial(amount int) string {
	raw := fmt.Sprintf("%d", amount)
	var chunks []string
	for len(raw) > 3 {
		chunks = append([]string{raw[len(raw)-3:]}, chunks...)
		raw = raw[:len(raw)-3]
	}
	return strings.Join(append([]string{raw}, chunks...), "٬")
}

func managerDashboard(re *core.RequestEvent) error {
	manager, err := requireManager(re)
	if err != nil {
		return err
	}
	csrf := csrfToken(re)
	players, err := findPlayers(re.App, "")
	if err != nil {
		return re.InternalServerError("بارگذاری بازیکنان ناموفق بود.", err)
	}
	guardianList, err := findGuardians(re.App)
	if err != nil {
		return re.InternalServerError("بارگذاری خانواده‌ها ناموفق بود.", err)
	}
	plans, err := findPlans(re.App)
	if err != nil {
		return re.InternalServerError("بارگذاری طرح‌های شهریه ناموفق بود.", err)
	}
	guardians, _ := re.App.CountRecords("guardians")
	active, _ := countRecordsByFilter(re.App, "subscriptions", "status = 'active'")
	due, _ := countRecordsByFilter(re.App, "invoices", "status = 'due' || status = 'overdue'")

	body := `<section class="page-heading"><div><p class="eyebrow">مدیریت باشگاه</p><h1>میزکار روزانه</h1><p>بازیکنان، خانواده‌ها، وضعیت اشتراک و شهریه را بدون پیچیدگی اضافی مدیریت کنید.</p></div><form method="post" action="/logout">` + hiddenCSRF(csrf) + `<button class="button button-quiet">خروج</button></form></section><section class="stats-grid"><article class="stat"><span>بازیکنان</span><strong>` + formatRial(len(players)) + `</strong></article><article class="stat"><span>سرپرستان</span><strong>` + formatRial(int(guardians)) + `</strong></article><article class="stat"><span>اشتراک فعال</span><strong>` + formatRial(int(active)) + `</strong><small>صورتحساب سررسید/معوق: ` + formatRial(int(due)) + `</small></article></section><section class="manager-grid"><div class="panel"><div class="panel-heading"><div><p class="eyebrow">فهرست بازیکنان</p><h2>جست‌وجو و پرونده‌ها</h2></div></div><input id="player-search" class="search" name="q" placeholder="جست‌وجو با نام یا شماره باشگاهی" autocomplete="off" hx-get="/manager/players" hx-trigger="input changed delay:250ms" hx-target="#players-table-body" hx-swap="innerHTML"><div class="table-wrap"><table><thead><tr><th>بازیکن</th><th>شماره</th><th>وضعیت</th><th></th></tr></thead><tbody id="players-table-body">` + playerTableRows(players) + `</tbody></table></div></div><aside class="side-stack"><section class="panel"><p class="eyebrow">افزودن بازیکن</p><h2>ثبت سریع</h2><form hx-post="/manager/players" hx-target="#players-table-body" hx-swap="afterbegin" hx-on::after-request="if(event.detail.successful){this.reset()}">` + hiddenCSRF(csrf) + `<div class="field"><label>نام</label><input name="first_name" required></div><div class="field"><label>نام خانوادگی</label><input name="last_name" required></div><div class="field"><label>شماره باشگاهی</label><input name="club_number" placeholder="ARA-181" required></div><div class="field"><label>تاریخ تولد جلالی</label><input name="birth_date_jalali" placeholder="1392/01/01"></div><div class="field"><label>وضعیت پرونده</label><select name="status"><option value="active">فعال</option><option value="suspended">تعلیق</option><option value="expired">منقضی</option></select></div><button class="button" type="submit">ثبت بازیکن</button></form></section>` + managerOperationsForm(csrf, players, plans) + managerBillingForm(csrf, players, guardianList) + reminderQueueHTML(re.App, csrf) + `<section id="player-detail" class="panel"><div class="empty compact">برای مشاهدهٔ جزئیات، یک بازیکن را انتخاب کنید.</div></section></aside></section>`
	return re.HTML(http.StatusOK, pageShell("مدیریت", manager.GetString("display_name"), "manager", body))
}
