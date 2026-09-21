package app

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

type guardianSummary struct {
	ID       string
	FullName string
	Mobile   string
}

func registerManagerBillingRoutes(e *core.ServeEvent) {
	e.Router.POST("/manager/guardians", createGuardianLink)
	e.Router.POST("/manager/invoices", createInvoice)
	e.Router.POST("/manager/plans", createPlan)
	e.Router.POST("/manager/subscriptions", enrollPlayer)
	e.Router.POST("/manager/players/{id}/status", updatePlayerStatus)
	e.Router.POST("/manager/reminders/{id}/complete", completeReminder)
}

func createGuardianLink(re *core.RequestEvent) error {
	manager, err := requireManager(re)
	if err != nil {
		return err
	}
	if err := validateCSRF(re); err != nil {
		return err
	}
	fullName := compactText(re.Request.FormValue("full_name"))
	mobile := normalizeMobile(re.Request.FormValue("mobile"))
	password := re.Request.FormValue("password")
	playerID := strings.TrimSpace(re.Request.FormValue("player_id"))
	relationship := compactText(re.Request.FormValue("relationship"))
	if fullName == "" || mobile == "" || playerID == "" || relationship == "" {
		return re.BadRequestError("نام، شماره همراه، بازیکن و نسبت الزامی است.", nil)
	}
	if _, err := re.App.FindRecordById("players", playerID); err != nil {
		return re.BadRequestError("بازیکن انتخاب‌شده معتبر نیست.", nil)
	}

	guardian, err := re.App.FindFirstRecordByFilter("guardians", "mobile = {:mobile}", map[string]any{"mobile": mobile})
	if err != nil {
		if len(password) < 8 {
			return re.BadRequestError("برای حساب جدید خانواده، رمز عبور حداقل ۸ کاراکتر است.", nil)
		}
		collection, collectionErr := re.App.FindCollectionByNameOrId("guardians")
		if collectionErr != nil {
			return re.InternalServerError("مجموعهٔ خانواده‌ها در دسترس نیست.", collectionErr)
		}
		guardian = core.NewRecord(collection)
		guardian.SetEmail(mobile + "@guardian.pabetoop.local")
		guardian.SetPassword(password)
		guardian.Set("full_name", fullName)
		guardian.Set("mobile", mobile)
		if saveErr := re.App.Save(guardian); saveErr != nil {
			return re.BadRequestError("ثبت حساب خانواده ناموفق بود؛ شماره همراه باید یکتا باشد.", nil)
		}
	}

	links, err := re.App.FindRecordsByFilter("guardian_players", "guardian = {:guardian} && player = {:player}", "", 1, 0, map[string]any{"guardian": guardian.Id, "player": playerID})
	if err == nil && len(links) > 0 {
		return re.BadRequestError("این ارتباط خانواده و بازیکن از قبل ثبت شده است.", nil)
	}
	collection, err := re.App.FindCollectionByNameOrId("guardian_players")
	if err != nil {
		return re.InternalServerError("مجموعهٔ ارتباط خانواده و بازیکن در دسترس نیست.", err)
	}
	link := core.NewRecord(collection)
	link.Set("guardian", guardian.Id)
	link.Set("player", playerID)
	link.Set("relationship", relationship)
	link.Set("is_verified", true)
	if err := re.App.Save(link); err != nil {
		return re.InternalServerError("اتصال خانواده به بازیکن ناموفق بود.", err)
	}
	_ = writeAudit(re.App, manager.Id, "guardian_player.created", "guardian_player", link.Id, map[string]any{"guardian": guardian.Id, "player": playerID})
	return re.Redirect(http.StatusSeeOther, "/manager")
}

func createInvoice(re *core.RequestEvent) error {
	manager, err := requireManager(re)
	if err != nil {
		return err
	}
	if err := validateCSRF(re); err != nil {
		return err
	}
	guardianID := strings.TrimSpace(re.Request.FormValue("guardian_id"))
	playerID := strings.TrimSpace(re.Request.FormValue("player_id"))
	amount, err := strconv.Atoi(strings.TrimSpace(re.Request.FormValue("amount_rial")))
	dueAt, dueErr := parseISODate(re.Request.FormValue("due_on"), nowUTC())
	if guardianID == "" || playerID == "" || err != nil || dueErr != nil || amount < 10000 {
		return re.BadRequestError("خانواده، بازیکن و مبلغ حداقل ۱۰٬۰۰۰ ریال الزامی است.", nil)
	}
	allowed, err := guardianCanAccessPlayer(re.App, guardianID, playerID)
	if err != nil || !allowed {
		return re.BadRequestError("خانوادهٔ انتخاب‌شده به این بازیکن متصل نیست.", nil)
	}
	if _, err := re.App.FindRecordById("guardians", guardianID); err != nil {
		return re.BadRequestError("خانوادهٔ انتخاب‌شده معتبر نیست.", nil)
	}
	if _, err := re.App.FindRecordById("players", playerID); err != nil {
		return re.BadRequestError("بازیکن انتخاب‌شده معتبر نیست.", nil)
	}
	collection, err := re.App.FindCollectionByNameOrId("invoices")
	if err != nil {
		return re.InternalServerError("مجموعهٔ صورتحساب‌ها در دسترس نیست.", err)
	}
	invoice := core.NewRecord(collection)
	invoice.Set("guardian", guardianID)
	invoice.Set("player", playerID)
	if subscription, _ := latestSubscription(re.App, playerID); subscription != nil {
		invoice.Set("subscription", subscription.Id)
	}
	invoice.Set("amount_rial", amount)
	invoice.Set("due_at", dueAt.Format(types.DefaultDateLayout))
	invoice.Set("status", "due")
	invoice.Set("plan_snapshot", "شهریه صادرشده توسط مدیریت")
	if err := re.App.Save(invoice); err != nil {
		return re.InternalServerError("صدور صورتحساب ناموفق بود.", err)
	}
	_ = writeAudit(re.App, manager.Id, "invoice.created", "invoice", invoice.Id, map[string]any{"guardian": guardianID, "player": playerID, "amount_rial": amount, "due_at": dueAt.Format(types.DefaultDateLayout)})
	return re.Redirect(http.StatusSeeOther, "/manager")
}

func findGuardians(app core.App) ([]guardianSummary, error) {
	records, err := app.FindRecordsByFilter("guardians", "", "", 500, 0)
	if err != nil {
		return nil, err
	}
	guardians := make([]guardianSummary, 0, len(records))
	for _, record := range records {
		guardians = append(guardians, guardianSummary{ID: record.Id, FullName: record.GetString("full_name"), Mobile: record.GetString("mobile")})
	}
	return guardians, nil
}

func playerOptions(players []playerSummary) string {
	var options strings.Builder
	for _, player := range players {
		options.WriteString(`<option value="` + escape(player.ID) + `">` + escape(player.FirstName+" "+player.LastName+" · "+player.ClubNumber) + `</option>`)
	}
	return options.String()
}

func guardianOptions(guardians []guardianSummary) string {
	var options strings.Builder
	for _, guardian := range guardians {
		options.WriteString(`<option value="` + escape(guardian.ID) + `">` + escape(guardian.FullName+" · "+guardian.Mobile) + `</option>`)
	}
	return options.String()
}

func managerBillingForm(csrf string, players []playerSummary, guardians []guardianSummary) string {
	playerSelect := playerOptions(players)
	guardianSelect := guardianOptions(guardians)
	today := nowUTC().Format("2006-01-02")
	return `<section class="panel"><p class="eyebrow">خانواده و دسترسی</p><h2>ثبت و اتصال خانواده</h2><form method="post" action="/manager/guardians">` + hiddenCSRF(csrf) + `<div class="field"><label>نام سرپرست</label><input name="full_name" required></div><div class="field"><label>شماره همراه</label><input name="mobile" inputmode="tel" placeholder="0917…" required></div><div class="field"><label>رمز موقت (فقط حساب جدید)</label><input name="password" type="password" minlength="8"></div><div class="field"><label>بازیکن</label><select name="player_id" required><option value="">انتخاب کنید</option>` + playerSelect + `</select></div><div class="field"><label>نسبت</label><input name="relationship" value="والد" required></div><button class="button" type="submit">ثبت و اتصال</button></form></section><section class="panel"><p class="eyebrow">صورتحساب</p><h2>صدور شهریه</h2><form method="post" action="/manager/invoices">` + hiddenCSRF(csrf) + `<div class="field"><label>خانواده</label><select name="guardian_id" required><option value="">انتخاب کنید</option>` + guardianSelect + `</select></div><div class="field"><label>بازیکنِ متصل</label><select name="player_id" required><option value="">انتخاب کنید</option>` + playerSelect + `</select></div><div class="field"><label>مبلغ ریال</label><input name="amount_rial" inputmode="numeric" min="10000" placeholder="15000000" required></div><div class="field"><label>مهلت پرداخت</label><input name="due_on" type="date" value="` + today + `" required></div><button class="button" type="submit">صدور صورتحساب</button></form></section>`
}

func managerBillingSummary(guardians []guardianSummary) string {
	return fmt.Sprintf("%d خانوادهٔ ثبت‌شده", len(guardians))
}
