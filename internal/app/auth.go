package app

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/shaiinarab/pabetoop-club/internal/brand"
)

const sessionCookieName = "pabetoop_session"

// registerAuthRoutes intentionally uses PocketBase auth records only to mint
// the signed session token. The browser never receives permission to list or
// mutate operational collections directly.
func registerAuthRoutes(e *core.ServeEvent) {
	e.Router.GET("/login", func(re *core.RequestEvent) error {
		return re.HTML(http.StatusOK, loginHTML("", csrfToken(re)))
	})

	e.Router.POST("/login", func(re *core.RequestEvent) error {
		if !loginLimiter.allow(re.RealIP(), time.Now()) {
			return re.TooManyRequestsError("تلاش‌های ورود بیش از حد مجاز است. چند دقیقه بعد دوباره کوشش کنید.", nil)
		}
		if err := validateCSRF(re); err != nil {
			return err
		}
		identifier := strings.TrimSpace(re.Request.FormValue("identifier"))
		password := re.Request.FormValue("password")
		record, err := authenticate(re.App, identifier, password)
		if err != nil {
			return re.HTML(http.StatusUnauthorized, loginHTML("شماره همراه یا رمز عبور صحیح نیست.", csrfToken(re)))
		}

		token, err := record.NewAuthToken()
		if err != nil {
			return re.InternalServerError("صدور نشست ورود ناموفق بود.", err)
		}
		re.SetCookie(&http.Cookie{
			Name:     sessionCookieName,
			Value:    token,
			Path:     "/",
			MaxAge:   int((7 * 24 * time.Hour).Seconds()),
			HttpOnly: true,
			Secure:   re.IsTLS(),
			SameSite: http.SameSiteLaxMode,
		})
		return re.Redirect(http.StatusSeeOther, landingPath(record))
	})

	e.Router.POST("/logout", func(re *core.RequestEvent) error {
		if err := validateCSRF(re); err != nil {
			return err
		}
		re.SetCookie(&http.Cookie{
			Name:     sessionCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   re.IsTLS(),
			SameSite: http.SameSiteLaxMode,
		})
		return re.Redirect(http.StatusSeeOther, "/")
	})
}

func authenticate(app core.App, identifier, password string) (*core.Record, error) {
	if identifier == "" || password == "" {
		return nil, errors.New("missing credentials")
	}

	if guardian, err := app.FindFirstRecordByFilter(
		"guardians",
		"mobile = {:mobile}",
		map[string]any{"mobile": normalizeMobile(identifier)},
	); err == nil && guardian.ValidatePassword(password) {
		return guardian, nil
	}

	staff, err := app.FindAuthRecordByEmail("staff", strings.ToLower(identifier))
	if err != nil || !staff.ValidatePassword(password) {
		return nil, errors.New("invalid credentials")
	}
	return staff, nil
}

func currentSession(app core.App, request *http.Request) (*core.Record, error) {
	cookie, err := request.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}
	return app.FindAuthRecordByToken(cookie.Value, core.TokenTypeAuth)
}

func isManager(record *core.Record) bool {
	if record == nil || record.Collection().Name != "staff" {
		return false
	}
	role := record.GetString("role")
	return role == "manager" || role == "admin"
}

func guardianCanAccessPlayer(app core.App, guardianID, playerID string) (bool, error) {
	relations, err := app.FindRecordsByFilter(
		"guardian_players",
		"guardian = {:guardian} && player = {:player} && is_verified = true",
		"",
		1,
		0,
		map[string]any{"guardian": guardianID, "player": playerID},
	)
	if err != nil {
		return false, err
	}
	return len(relations) == 1, nil
}

func normalizeMobile(input string) string {
	normalized := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(input)
	if strings.HasPrefix(normalized, "+98") {
		return "0" + strings.TrimPrefix(normalized, "+98")
	}
	if strings.HasPrefix(normalized, "98") && len(normalized) == 12 {
		return "0" + strings.TrimPrefix(normalized, "98")
	}
	return normalized
}

func landingPath(record *core.Record) string {
	if isManager(record) {
		return "/manager"
	}
	return "/portal"
}

func loginHTML(message, token string) string {
	alert := ""
	if message != "" {
		alert = `<p role="alert" class="alert">` + message + `</p>`
	}
	return `<!doctype html><html lang="fa" dir="rtl"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>ورود | ` + escape(brand.Active().NameFA) + `</title><style>body{background:#f5f7f3;color:#173325;font-family:Tahoma,sans-serif;display:grid;place-items:center;min-height:100vh;margin:0}.panel{width:min(420px,calc(100% - 2rem));background:white;border-radius:18px;padding:2rem;box-shadow:0 15px 40px #17332518}label{display:grid;gap:.45rem;margin:1rem 0}input{padding:.8rem;border:1px solid #b8c6bd;border-radius:9px;font:inherit}.button{width:100%;padding:.8rem;border:0;border-radius:9px;background:#103f2f;color:white;font:inherit;font-weight:bold}.alert{background:#fff0f0;color:#9a2424;border-radius:8px;padding:.75rem}</style></head><body><main class="panel"><p>` + escape(brand.Active().TitleFA()) + `</p><h1>ورود خانواده و مدیریت</h1>` + alert + `<form method="post" action="/login">` + hiddenCSRF(token) + `<label>شماره همراه سرپرست یا ایمیل مدیر<input name="identifier" autocomplete="username" required></label><label>رمز عبور<input name="password" type="password" autocomplete="current-password" required></label><button class="button" type="submit">ورود امن</button></form></main></body></html>`
}
