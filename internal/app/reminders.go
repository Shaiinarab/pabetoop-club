package app

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	reminderJobID  = "pabetoop-invoice-reminder-scan"
	reminderKeyDue = "invoice_due_v1"
)

func registerReminderAutomation(e *core.ServeEvent) error {
	return e.App.Cron().Add(reminderJobID, "0 7 * * *", func() {
		created, err := enqueueDueInvoiceReminders(e.App, nowUTC())
		if err != nil {
			e.App.Logger().Error("invoice reminder scan failed", "error", err)
			return
		}
		e.App.Logger().Info("invoice reminder scan completed", "created", created)
	})
}

// enqueueDueInvoiceReminders creates manager-visible operational work items. It
// deliberately does not contact external messaging providers: a real provider
// needs explicit family consent, delivery credentials and failure recovery.
func enqueueDueInvoiceReminders(app core.App, now time.Time) (int, error) {
	if err := markOverdueInvoices(app, now); err != nil {
		return 0, err
	}
	invoices, err := app.FindRecordsByFilter(
		"invoices",
		"(status = 'due' || status = 'overdue') && due_at <= {:now}",
		"due_at",
		500,
		0,
		map[string]any{"now": now.UTC().Format(types.DefaultDateLayout)},
	)
	if err != nil {
		return 0, fmt.Errorf("list due invoices: %w", err)
	}
	collection, err := app.FindCollectionByNameOrId("reminder_jobs")
	if err != nil {
		return 0, fmt.Errorf("load reminder_jobs collection: %w", err)
	}

	created := 0
	for _, invoice := range invoices {
		existing, err := app.FindRecordsByFilter(
			"reminder_jobs",
			"invoice = {:invoice} && reminder_key = {:key}",
			"",
			1,
			0,
			map[string]any{"invoice": invoice.Id, "key": reminderKeyDue},
		)
		if err != nil {
			return created, fmt.Errorf("find reminder for invoice %s: %w", invoice.Id, err)
		}
		if len(existing) > 0 {
			continue
		}

		reminder := core.NewRecord(collection)
		reminder.Set("invoice", invoice.Id)
		reminder.Set("guardian", invoice.GetString("guardian"))
		reminder.Set("status", "queued")
		reminder.Set("channel", "manager_queue")
		reminder.Set("reminder_key", reminderKeyDue)
		reminder.Set("scheduled_for", now.UTC().Format(types.DefaultDateLayout))
		reminder.Set("attempt_count", 0)
		if err := app.Save(reminder); err != nil {
			return created, fmt.Errorf("create reminder for invoice %s: %w", invoice.Id, err)
		}
		created++
	}
	return created, nil
}

func markOverdueInvoices(app core.App, now time.Time) error {
	invoices, err := app.FindRecordsByFilter(
		"invoices",
		"status = 'due' && due_at < {:now}",
		"",
		500,
		0,
		map[string]any{"now": now.UTC().Format(types.DefaultDateLayout)},
	)
	if err != nil {
		return fmt.Errorf("list invoices to mark overdue: %w", err)
	}
	for _, invoice := range invoices {
		invoice.Set("status", "overdue")
		if err := app.Save(invoice); err != nil {
			return fmt.Errorf("mark invoice %s overdue: %w", invoice.Id, err)
		}
	}
	return nil
}

func completeReminder(re *core.RequestEvent) error {
	manager, err := requireManager(re)
	if err != nil {
		return err
	}
	if err := validateCSRF(re); err != nil {
		return err
	}
	reminder, err := re.App.FindRecordById("reminder_jobs", re.Request.PathValue("id"))
	if err != nil {
		return re.NotFoundError("یادآوری پیدا نشد.", err)
	}
	if reminder.GetString("status") != "completed" {
		reminder.Set("status", "completed")
		reminder.Set("completed_at", nowUTC().Format(types.DefaultDateLayout))
		reminder.Set("attempt_count", reminder.GetInt("attempt_count")+1)
		if err := re.App.Save(reminder); err != nil {
			return re.InternalServerError("ثبت پیگیری ناموفق بود.", err)
		}
		_ = writeAudit(re.App, manager.Id, "reminder.completed", "reminder_job", reminder.Id, map[string]any{"invoice": reminder.GetString("invoice")})
	}
	return re.Redirect(http.StatusSeeOther, "/manager")
}

func reminderQueueHTML(app core.App, csrf string) string {
	jobs, err := app.FindRecordsByFilter("reminder_jobs", "status = 'queued'", "scheduled_for", 10, 0)
	if err != nil {
		return `<section class="panel"><p class="eyebrow">یادآوری شهریه</p><p class="alert">صف یادآوری فعلاً قابل بارگذاری نیست.</p></section>`
	}
	if len(jobs) == 0 {
		return `<section class="panel"><p class="eyebrow">یادآوری شهریه</p><h2>صف پیگیری</h2><div class="empty compact">در حال حاضر پیگیری سررسیدشده‌ای وجود ندارد.</div></section>`
	}
	var rows strings.Builder
	for _, job := range jobs {
		invoice, invoiceErr := app.FindRecordById("invoices", job.GetString("invoice"))
		amount := "—"
		if invoiceErr == nil {
			amount = formatRial(invoice.GetInt("amount_rial")) + " ریال"
		}
		rows.WriteString(`<li><div><strong>` + escape(amount) + `</strong><span>پیگیری دستی موردنیاز</span></div><form method="post" action="/manager/reminders/` + escape(job.Id) + `/complete">` + hiddenCSRF(csrf) + `<button class="button button-small" type="submit">ثبت پیگیری</button></form></li>`)
	}
	return `<section class="panel reminder-panel"><p class="eyebrow">یادآوری شهریه</p><h2>صف پیگیری</h2><p class="muted">تا زمانی که کانال ارسال و رضایت خانواده فعال نشده، این‌ها فقط کارهای قابل اقدام مدیر هستند.</p><ul class="reminder-list">` + rows.String() + `</ul></section>`
}
