package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/shaiinarab/pabetoop-club/internal/brand"
)

type paymentStartRequest struct {
	AmountRial  int
	Description string
	Mobile      string
	CallbackURL string
}

type paymentStartResponse struct {
	Authority   string
	RedirectURL string
}

type paymentVerifyResponse struct {
	Reference string
	Summary   map[string]any
}

type paymentGateway interface {
	Name() string
	Start(ctx context.Context, request paymentStartRequest) (paymentStartResponse, error)
	Verify(ctx context.Context, authority string, amountRial int) (paymentVerifyResponse, error)
}

type testGateway struct{}

func (testGateway) Name() string { return "test" }
func (testGateway) Start(_ context.Context, _ paymentStartRequest) (paymentStartResponse, error) {
	return paymentStartResponse{Authority: "TST-" + secureToken()}, nil
}
func (testGateway) Verify(_ context.Context, authority string, _ int) (paymentVerifyResponse, error) {
	if !strings.HasPrefix(authority, "TST-") {
		return paymentVerifyResponse{}, errors.New("invalid test authority")
	}
	return paymentVerifyResponse{Reference: "TEST-" + authority, Summary: map[string]any{"code": 100, "mode": "test"}}, nil
}

type zarinpalGateway struct {
	merchantID string
	sandbox    bool
	client     *http.Client
}

func (gateway zarinpalGateway) Name() string { return "zarinpal" }
func (gateway zarinpalGateway) endpoint(path string) string {
	host := "https://payment.zarinpal.com"
	if gateway.sandbox {
		host = "https://sandbox.zarinpal.com"
	}
	return host + path
}

func (gateway zarinpalGateway) Start(ctx context.Context, request paymentStartRequest) (paymentStartResponse, error) {
	payload := map[string]any{
		"merchant_id":  gateway.merchantID,
		"amount":       request.AmountRial,
		"callback_url": request.CallbackURL,
		"description":  request.Description,
		"currency":     "IRR",
		"metadata":     map[string]any{"mobile": request.Mobile},
	}
	var response struct {
		Data struct {
			Code      int    `json:"code"`
			Authority string `json:"authority"`
		} `json:"data"`
		Errors any `json:"errors"`
	}
	if err := gateway.postJSON(ctx, "/pg/v4/payment/request.json", payload, &response); err != nil {
		return paymentStartResponse{}, err
	}
	if response.Data.Code != 100 || response.Data.Authority == "" {
		return paymentStartResponse{}, fmt.Errorf("zarinpal request rejected: %v", response.Errors)
	}
	return paymentStartResponse{Authority: response.Data.Authority, RedirectURL: gateway.endpoint("/pg/StartPay/" + url.PathEscape(response.Data.Authority))}, nil
}

func (gateway zarinpalGateway) Verify(ctx context.Context, authority string, amountRial int) (paymentVerifyResponse, error) {
	payload := map[string]any{"merchant_id": gateway.merchantID, "amount": amountRial, "authority": authority, "currency": "IRR"}
	var response struct {
		Data struct {
			Code    int    `json:"code"`
			RefID   int64  `json:"ref_id"`
			CardPAN string `json:"card_pan"`
			Fee     int    `json:"fee"`
			FeeType string `json:"fee_type"`
		} `json:"data"`
		Errors any `json:"errors"`
	}
	if err := gateway.postJSON(ctx, "/pg/v4/payment/verify.json", payload, &response); err != nil {
		return paymentVerifyResponse{}, err
	}
	// 100 indicates a new verification and 101 a previously verified payment.
	if response.Data.Code != 100 && response.Data.Code != 101 {
		return paymentVerifyResponse{}, fmt.Errorf("zarinpal verification rejected: %v", response.Errors)
	}
	return paymentVerifyResponse{Reference: fmt.Sprintf("%d", response.Data.RefID), Summary: map[string]any{"code": response.Data.Code, "card_pan": response.Data.CardPAN, "fee": response.Data.Fee, "fee_type": response.Data.FeeType}}, nil
}

func (gateway zarinpalGateway) postJSON(ctx context.Context, path string, payload any, destination any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, gateway.endpoint(path), bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	client := gateway.client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("zarinpal HTTP status %d", response.StatusCode)
	}
	return json.NewDecoder(response.Body).Decode(destination)
}

func registerPaymentRoutes(e *core.ServeEvent) {
	e.Router.GET("/pay/{id}", paymentPage)
	e.Router.POST("/pay/{id}/start", startPayment)
	e.Router.GET("/pay/test/{id}", testPaymentPage)
	e.Router.POST("/pay/test/{id}/complete", completeTestPayment)
	e.Router.GET("/payments/callback", paymentCallback)
}

func paymentPage(re *core.RequestEvent) error {
	guardian, invoice, err := guardianInvoice(re)
	if err != nil {
		return err
	}
	if invoice.GetString("status") == "paid" {
		return re.HTML(http.StatusOK, paymentResultHTML("پرداخت قبلاً ثبت شده است", "این صورتحساب پیش‌تر تسویه شده است.", "/portal"))
	}
	csrf := csrfToken(re)
	player, err := re.App.FindRecordById("players", invoice.GetString("player"))
	if err != nil {
		return re.InternalServerError("پروندهٔ بازیکن در دسترس نیست.", err)
	}
	provider := selectedGatewayName()
	return re.HTML(http.StatusOK, pageShell("پرداخت شهریه", guardian.GetString("full_name"), "payment", `<section class="payment-wrap"><article class="panel payment-card"><p class="eyebrow">پرداخت شهریه</p><h1>`+playerName(player)+`</h1><p class="muted">مبلغ فقط از صورتحساب ثبت‌شده خوانده می‌شود و در مرورگر قابل تغییر نیست.</p><div class="amount-box"><span>مبلغ قابل پرداخت</span><strong>`+formatRial(invoice.GetInt("amount_rial"))+` ریال</strong></div><dl class="receipt-meta"><div><dt>شناسهٔ صورتحساب</dt><dd>`+escape(invoice.Id)+`</dd></div><div><dt>درگاه انتخاب‌شده</dt><dd>`+gatewayLabel(provider)+`</dd></div></dl><form method="post" action="/pay/`+escape(invoice.Id)+`/start">`+hiddenCSRF(csrf)+`<button class="button" type="submit">ادامه به پرداخت</button></form><a class="text-link" href="/portal">بازگشت به پورتال</a></article></section>`))
}

func startPayment(re *core.RequestEvent) error {
	guardian, invoice, err := guardianInvoice(re)
	if err != nil {
		return err
	}
	if err := validateCSRF(re); err != nil {
		return err
	}
	if invoice.GetString("status") == "paid" {
		return re.Redirect(http.StatusSeeOther, "/pay/"+invoice.Id)
	}
	if invoice.GetString("status") != "due" && invoice.GetString("status") != "overdue" {
		return re.BadRequestError("این صورتحساب در وضعیت قابل پرداخت نیست.", nil)
	}

	provider, err := selectedGateway()
	if err != nil {
		return re.InternalServerError("تنظیمات درگاه پرداخت کامل نیست.", err)
	}
	collection, err := re.App.FindCollectionByNameOrId("payments")
	if err != nil {
		return re.InternalServerError("ثبت پرداخت در دسترس نیست.", err)
	}
	payment := core.NewRecord(collection)
	payment.Set("invoice", invoice.Id)
	payment.Set("provider", provider.Name())
	payment.Set("authority", "PENDING-"+secureToken())
	payment.Set("amount_rial", invoice.GetInt("amount_rial"))
	payment.Set("status", "initiated")
	if err := re.App.Save(payment); err != nil {
		return re.InternalServerError("ایجاد پرداخت ناموفق بود.", err)
	}

	callbackURL := callbackURLForPayment(payment.Id)
	result, err := provider.Start(re.Request.Context(), paymentStartRequest{
		AmountRial:  invoice.GetInt("amount_rial"),
		Description: "شهریه " + brand.Active().TitleFA() + " - " + invoice.Id,
		Mobile:      guardian.GetString("mobile"),
		CallbackURL: callbackURL,
	})
	if err != nil {
		payment.Set("status", "failed")
		payment.Set("verification_summary", map[string]any{"reason": "gateway_start_failed"})
		_ = re.App.Save(payment)
		return re.InternalServerError("شروع اتصال به درگاه ناموفق بود. لطفاً دوباره تلاش کنید.", err)
	}
	payment.Set("authority", result.Authority)
	if err := re.App.Save(payment); err != nil {
		return re.InternalServerError("ثبت شناسهٔ پرداخت ناموفق بود.", err)
	}
	_ = writeAudit(re.App, guardian.Id, "payment.initiated", "payment", payment.Id, map[string]any{"invoice": invoice.Id, "provider": provider.Name()})

	if provider.Name() == "test" {
		return re.Redirect(http.StatusSeeOther, "/pay/test/"+payment.Id)
	}
	return re.Redirect(http.StatusSeeOther, result.RedirectURL)
}

func testPaymentPage(re *core.RequestEvent) error {
	guardian, payment, invoice, err := guardianPayment(re)
	if err != nil {
		return err
	}
	if payment.GetString("provider") != "test" {
		return re.NotFoundError("صفحهٔ آزمایشی پیدا نشد.", nil)
	}
	csrf := csrfToken(re)
	return re.HTML(http.StatusOK, pageShell("پرداخت آزمایشی", guardian.GetString("full_name"), "payment", `<section class="payment-wrap"><article class="panel payment-card"><p class="eyebrow">محیط آزمایشی</p><h1>تأیید پرداخت آزمایشی</h1><p>هیچ تراکنش بانکی انجام نمی‌شود. این مرحله فقط وضعیت پرداخت و دسترسی شما را بررسی می‌کند.</p><div class="amount-box"><span>مبلغ صورت‌حساب</span><strong>`+formatRial(invoice.GetInt("amount_rial"))+` ریال</strong></div><form method="post" action="/pay/test/`+escape(payment.Id)+`/complete">`+hiddenCSRF(csrf)+`<button class="button" type="submit">ثبت موفقیت پرداخت آزمایشی</button></form></article></section>`))
}

func completeTestPayment(re *core.RequestEvent) error {
	guardian, payment, invoice, err := guardianPayment(re)
	if err != nil {
		return err
	}
	if payment.GetString("provider") != "test" {
		return re.NotFoundError("پرداخت آزمایشی پیدا نشد.", nil)
	}
	if err := validateCSRF(re); err != nil {
		return err
	}
	result, err := testGateway{}.Verify(re.Request.Context(), payment.GetString("authority"), invoice.GetInt("amount_rial"))
	if err != nil {
		return re.InternalServerError("تأیید آزمایشی ناموفق بود.", err)
	}
	if err := settlePayment(re.App, payment, invoice, result); err != nil {
		return re.InternalServerError("ثبت تسویه ناموفق بود.", err)
	}
	_ = writeAudit(re.App, guardian.Id, "payment.verified", "payment", payment.Id, map[string]any{"invoice": invoice.Id, "provider": "test"})
	return re.HTML(http.StatusOK, paymentResultHTML("پرداخت آزمایشی ثبت شد", "صورتحساب شما تسویه شد و وضعیت اشتراک در پورتال به‌روز است.", "/portal"))
}

func paymentCallback(re *core.RequestEvent) error {
	paymentID := re.Request.URL.Query().Get("payment_id")
	payment, err := re.App.FindRecordById("payments", paymentID)
	if err != nil {
		return re.NotFoundError("پرداخت پیدا نشد.", err)
	}
	invoice, err := re.App.FindRecordById("invoices", payment.GetString("invoice"))
	if err != nil {
		return re.InternalServerError("صورتحساب پیدا نشد.", err)
	}
	if payment.GetString("status") == "verified" {
		return re.HTML(http.StatusOK, paymentResultHTML("پرداخت پیش‌تر تأیید شده است", "شناسهٔ پیگیری: "+escape(orDash(payment.GetString("reference"))), "/portal"))
	}
	if re.Request.URL.Query().Get("Status") != "OK" {
		payment.Set("status", "cancelled")
		_ = re.App.Save(payment)
		return re.HTML(http.StatusOK, paymentResultHTML("پرداخت کامل نشد", "درگاه پرداخت، تراکنش را ناموفق یا لغوشده گزارش کرد.", "/pay/"+invoice.Id))
	}
	if authority := re.Request.URL.Query().Get("Authority"); authority == "" || authority != payment.GetString("authority") {
		return re.BadRequestError("شناسهٔ بازگشت پرداخت معتبر نیست.", nil)
	}
	provider, err := selectedGatewayByName(payment.GetString("provider"))
	if err != nil {
		return re.InternalServerError("درگاه پرداخت در دسترس نیست.", err)
	}
	result, err := provider.Verify(re.Request.Context(), payment.GetString("authority"), invoice.GetInt("amount_rial"))
	if err != nil {
		payment.Set("status", "failed")
		_ = re.App.Save(payment)
		return re.HTML(http.StatusBadGateway, paymentResultHTML("تأیید پرداخت ناموفق بود", "مبلغ از اطلاعات صورتحساب کنترل شد، اما درگاه پرداخت تأیید نهایی را برنگرداند.", "/pay/"+invoice.Id))
	}
	if err := settlePayment(re.App, payment, invoice, result); err != nil {
		return re.InternalServerError("ثبت تأیید پرداخت ناموفق بود.", err)
	}
	return re.HTML(http.StatusOK, paymentResultHTML("پرداخت با موفقیت تأیید شد", "شناسهٔ پیگیری: "+escape(result.Reference), "/portal"))
}

func settlePayment(app core.App, payment, invoice *core.Record, result paymentVerifyResponse) error {
	if payment.GetString("status") == "verified" {
		return nil
	}
	payment.Set("status", "verified")
	payment.Set("reference", result.Reference)
	payment.Set("verification_summary", result.Summary)
	payment.Set("verified_at", time.Now().UTC().Format(time.RFC3339))
	invoice.Set("status", "paid")
	return app.RunInTransaction(func(txApp core.App) error {
		if err := txApp.Save(payment); err != nil {
			return err
		}
		if err := txApp.Save(invoice); err != nil {
			return err
		}
		if subscriptionID := invoice.GetString("subscription"); subscriptionID != "" {
			subscription, err := txApp.FindRecordById("subscriptions", subscriptionID)
			if err != nil {
				return err
			}
			subscription.Set("status", "active")
			subscription.Set("reason", "فعال‌سازی پس از تأیید پرداخت")
			if err := txApp.Save(subscription); err != nil {
				return err
			}
		}
		player, err := txApp.FindRecordById("players", invoice.GetString("player"))
		if err != nil {
			return err
		}
		if player.GetString("status") != "suspended" {
			player.Set("status", "active")
			if err := txApp.Save(player); err != nil {
				return err
			}
		}
		return nil
	})
}

func guardianInvoice(re *core.RequestEvent) (*core.Record, *core.Record, error) {
	guardian, err := currentSession(re.App, re.Request)
	if err != nil {
		return nil, nil, re.UnauthorizedError("برای پرداخت باید وارد شوید.", nil)
	}
	if guardian.Collection().Name != "guardians" {
		return nil, nil, re.ForbiddenError("پرداخت فقط برای سرپرست ثبت‌شده مجاز است.", nil)
	}
	invoice, err := re.App.FindRecordById("invoices", re.Request.PathValue("id"))
	if err != nil {
		return nil, nil, re.NotFoundError("صورتحساب پیدا نشد.", err)
	}
	if invoice.GetString("guardian") != guardian.Id {
		return nil, nil, re.ForbiddenError("این صورتحساب به حساب شما تعلق ندارد.", nil)
	}
	return guardian, invoice, nil
}

func guardianPayment(re *core.RequestEvent) (*core.Record, *core.Record, *core.Record, error) {
	guardian, err := currentSession(re.App, re.Request)
	if err != nil {
		return nil, nil, nil, re.UnauthorizedError("برای پرداخت باید وارد شوید.", nil)
	}
	if guardian.Collection().Name != "guardians" {
		return nil, nil, nil, re.ForbiddenError("پرداخت فقط برای سرپرست ثبت‌شده مجاز است.", nil)
	}
	payment, err := re.App.FindRecordById("payments", re.Request.PathValue("id"))
	if err != nil {
		return nil, nil, nil, re.NotFoundError("پرداخت پیدا نشد.", err)
	}
	invoice, err := re.App.FindRecordById("invoices", payment.GetString("invoice"))
	if err != nil {
		return nil, nil, nil, re.InternalServerError("صورتحساب پیدا نشد.", err)
	}
	if invoice.GetString("guardian") != guardian.Id {
		return nil, nil, nil, re.ForbiddenError("این پرداخت به حساب شما تعلق ندارد.", nil)
	}
	return guardian, payment, invoice, nil
}

func selectedGateway() (paymentGateway, error) { return selectedGatewayByName(selectedGatewayName()) }
func selectedGatewayName() string {
	if os.Getenv("STUB") == "1" || os.Getenv("PAYMENT_PROVIDER") == "" || os.Getenv("PAYMENT_PROVIDER") == "test" {
		return "test"
	}
	return strings.ToLower(os.Getenv("PAYMENT_PROVIDER"))
}
func selectedGatewayByName(name string) (paymentGateway, error) {
	switch name {
	case "test":
		return testGateway{}, nil
	case "zarinpal":
		merchantID := strings.TrimSpace(os.Getenv("ZARINPAL_MERCHANT_ID"))
		if merchantID == "" {
			return nil, errors.New("ZARINPAL_MERCHANT_ID is required")
		}
		return zarinpalGateway{merchantID: merchantID, sandbox: os.Getenv("ZARINPAL_SANDBOX") == "1"}, nil
	default:
		return nil, fmt.Errorf("unsupported payment provider %q", name)
	}
}
func callbackURLForPayment(paymentID string) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("ZARINPAL_CALLBACK_BASE_URL")), "/")
	if base == "" {
		base = "http://localhost:8090"
	}
	return base + "/payments/callback?payment_id=" + url.QueryEscape(paymentID)
}
func gatewayLabel(name string) string {
	if name == "zarinpal" {
		return "زرین‌پال"
	}
	return "درگاه آزمایشی"
}
func secureToken() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}
func paymentResultHTML(title, detail, href string) string {
	return `<!doctype html><html lang="fa" dir="rtl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + escape(title) + ` | ` + escape(brand.Active().NameFA) + `</title><style>` + appCSS + `</style></head><body><main class="main-content"><section class="payment-wrap"><article class="panel payment-card"><p class="eyebrow">` + escape(brand.Active().TitleFA()) + `</p><h1>` + escape(title) + `</h1><p>` + detail + `</p><a class="button" href="` + escape(href) + `">بازگشت</a></article></section></main></body></html>`
}
