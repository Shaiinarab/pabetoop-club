package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

type planSummary struct {
	ID           string
	Name         string
	AmountRial   int
	PeriodMonths int
	IsActive     bool
}

func createPlan(re *core.RequestEvent) error {
	manager, err := requireManager(re)
	if err != nil {
		return err
	}
	if err := validateCSRF(re); err != nil {
		return err
	}
	name := compactText(re.Request.FormValue("name"))
	amount, err := strconv.Atoi(strings.TrimSpace(re.Request.FormValue("amount_rial")))
	periodMonths, periodErr := strconv.Atoi(strings.TrimSpace(re.Request.FormValue("period_months")))
	if name == "" || err != nil || amount < 10000 || periodErr != nil || periodMonths < 1 || periodMonths > 12 {
		return re.BadRequestError("نام طرح، مبلغ حداقل ۱۰٬۰۰۰ ریال و دورهٔ ۱ تا ۱۲ ماه الزامی است.", nil)
	}
	collection, err := re.App.FindCollectionByNameOrId("plans")
	if err != nil {
		return re.InternalServerError("مجموعهٔ طرح‌ها در دسترس نیست.", err)
	}
	plan := core.NewRecord(collection)
	plan.Set("name", name)
	plan.Set("amount_rial", amount)
	plan.Set("period_months", periodMonths)
	plan.Set("is_active", true)
	if err := re.App.Save(plan); err != nil {
		return re.InternalServerError("ثبت طرح شهریه ناموفق بود.", err)
	}
	_ = writeAudit(re.App, manager.Id, "plan.created", "plan", plan.Id, map[string]any{"amount_rial": amount, "period_months": periodMonths})
	return re.Redirect(http.StatusSeeOther, "/manager")
}

func enrollPlayer(re *core.RequestEvent) error {
	manager, err := requireManager(re)
	if err != nil {
		return err
	}
	if err := validateCSRF(re); err != nil {
		return err
	}
	playerID := strings.TrimSpace(re.Request.FormValue("player_id"))
	planID := strings.TrimSpace(re.Request.FormValue("plan_id"))
	startsAt, err := parseISODate(re.Request.FormValue("starts_on"), nowUTC())
	if playerID == "" || planID == "" || err != nil {
		return re.BadRequestError("بازیکن، طرح و تاریخ شروع معتبر الزامی است.", nil)
	}
	player, err := re.App.FindRecordById("players", playerID)
	if err != nil {
		return re.BadRequestError("بازیکن انتخاب‌شده معتبر نیست.", nil)
	}
	plan, err := re.App.FindRecordById("plans", planID)
	if err != nil || !plan.GetBool("is_active") {
		return re.BadRequestError("طرح انتخاب‌شده فعال یا معتبر نیست.", nil)
	}
	if _, err := enrollPlayerInPlan(re.App, player, plan, manager.Id, startsAt); err != nil {
		return re.InternalServerError("ثبت اشتراک ناموفق بود.", err)
	}
	_ = writeAudit(re.App, manager.Id, "subscription.enrolled", "player", player.Id, map[string]any{"plan": plan.Id, "starts_at": startsAt.Format(types.DefaultDateLayout)})
	return re.Redirect(http.StatusSeeOther, "/manager")
}

func updatePlayerStatus(re *core.RequestEvent) error {
	manager, err := requireManager(re)
	if err != nil {
		return err
	}
	if err := validateCSRF(re); err != nil {
		return err
	}
	player, err := re.App.FindRecordById("players", re.Request.PathValue("id"))
	if err != nil {
		return re.NotFoundError("بازیکن پیدا نشد.", err)
	}
	status := normalizePlayerStatus(re.Request.FormValue("status"))
	if err := re.App.RunInTransaction(func(txApp core.App) error {
		player.Set("status", status)
		if err := txApp.Save(player); err != nil {
			return err
		}
		if subscription, _ := latestSubscription(txApp, player.Id); subscription != nil && status != "active" {
			subscription.Set("status", status)
			subscription.Set("reason", "به‌روزرسانی وضعیت پرونده توسط مدیر")
			subscription.Set("updated_by", manager.Id)
			return txApp.Save(subscription)
		}
		return nil
	}); err != nil {
		return re.InternalServerError("به‌روزرسانی وضعیت بازیکن ناموفق بود.", err)
	}
	_ = writeAudit(re.App, manager.Id, "player.status_updated", "player", player.Id, map[string]any{"status": status})
	return re.Redirect(http.StatusSeeOther, "/manager")
}

func enrollPlayerInPlan(app core.App, player, plan *core.Record, managerID string, startsAt time.Time) (*core.Record, error) {
	if plan.GetInt("period_months") < 1 {
		return nil, fmt.Errorf("plan %s has invalid period", plan.Id)
	}
	endsAt := startsAt.AddDate(0, plan.GetInt("period_months"), 0)
	collection, err := app.FindCollectionByNameOrId("subscriptions")
	if err != nil {
		return nil, err
	}
	if err := app.RunInTransaction(func(txApp core.App) error {
		activeSubscriptions, findErr := txApp.FindRecordsByFilter("subscriptions", "player = {:player} && status = 'active'", "", 20, 0, map[string]any{"player": player.Id})
		if findErr != nil {
			return findErr
		}
		for _, active := range activeSubscriptions {
			active.Set("status", "expired")
			active.Set("ends_at", startsAt.Format(types.DefaultDateLayout))
			active.Set("reason", "جایگزینی با ثبت‌نام جدید")
			active.Set("updated_by", managerID)
			if saveErr := txApp.Save(active); saveErr != nil {
				return saveErr
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	subscription := core.NewRecord(collection)
	subscription.Set("player", player.Id)
	subscription.Set("plan", plan.Id)
	subscription.Set("status", "active")
	subscription.Set("starts_at", startsAt.Format(types.DefaultDateLayout))
	subscription.Set("ends_at", endsAt.Format(types.DefaultDateLayout))
	subscription.Set("reason", "ثبت‌نام مدیر")
	subscription.Set("updated_by", managerID)
	if err := app.RunInTransaction(func(txApp core.App) error {
		if err := txApp.Save(subscription); err != nil {
			return err
		}
		player.Set("status", "active")
		return txApp.Save(player)
	}); err != nil {
		return nil, err
	}
	return subscription, nil
}

func parseISODate(raw string, fallback time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Date(fallback.UTC().Year(), fallback.UTC().Month(), fallback.UTC().Day(), 0, 0, 0, 0, time.UTC), nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func findPlans(app core.App) ([]planSummary, error) {
	records, err := app.FindRecordsByFilter("plans", "", "name", 100, 0)
	if err != nil {
		return nil, err
	}
	plans := make([]planSummary, 0, len(records))
	for _, record := range records {
		plans = append(plans, planSummary{
			ID:           record.Id,
			Name:         record.GetString("name"),
			AmountRial:   record.GetInt("amount_rial"),
			PeriodMonths: record.GetInt("period_months"),
			IsActive:     record.GetBool("is_active"),
		})
	}
	return plans, nil
}

func planOptions(plans []planSummary) string {
	var options strings.Builder
	for _, plan := range plans {
		if !plan.IsActive {
			continue
		}
		label := fmt.Sprintf("%s · %s ریال · %d ماه", plan.Name, formatRial(plan.AmountRial), plan.PeriodMonths)
		options.WriteString(`<option value="` + escape(plan.ID) + `">` + escape(label) + `</option>`)
	}
	return options.String()
}

func managerOperationsForm(csrf string, players []playerSummary, plans []planSummary) string {
	playerSelect := playerOptions(players)
	planSelect := planOptions(plans)
	today := nowUTC().Format("2006-01-02")
	planRows := `<div class="empty compact">هنوز طرح فعالی تعریف نشده است.</div>`
	if len(plans) > 0 {
		var rows strings.Builder
		for _, plan := range plans {
			state := "غیرفعال"
			if plan.IsActive {
				state = "فعال"
			}
			rows.WriteString(`<li><strong>` + escape(plan.Name) + `</strong><span>` + formatRial(plan.AmountRial) + ` ریال · ` + strconv.Itoa(plan.PeriodMonths) + ` ماه · ` + state + `</span></li>`)
		}
		planRows = `<ul class="plan-list">` + rows.String() + `</ul>`
	}
	return `<section class="panel"><p class="eyebrow">طرح شهریه</p><h2>تعریف طرح</h2><form method="post" action="/manager/plans">` + hiddenCSRF(csrf) + `<div class="field"><label>نام طرح</label><input name="name" placeholder="نوجوانان پایه" required></div><div class="field"><label>مبلغ ریال</label><input name="amount_rial" inputmode="numeric" min="10000" placeholder="18000000" required></div><div class="field"><label>مدت (ماه)</label><input name="period_months" inputmode="numeric" min="1" max="12" value="1" required></div><button class="button" type="submit">ثبت طرح</button></form><div class="plan-summary">` + planRows + `</div></section><section class="panel"><p class="eyebrow">ثبت‌نام</p><h2>فعال‌سازی اشتراک</h2><form method="post" action="/manager/subscriptions">` + hiddenCSRF(csrf) + `<div class="field"><label>بازیکن</label><select name="player_id" required><option value="">انتخاب کنید</option>` + playerSelect + `</select></div><div class="field"><label>طرح فعال</label><select name="plan_id" required><option value="">انتخاب کنید</option>` + planSelect + `</select></div><div class="field"><label>شروع اشتراک</label><input name="starts_on" type="date" value="` + today + `" required></div><button class="button" type="submit">ثبت اشتراک</button></form></section>`
}
