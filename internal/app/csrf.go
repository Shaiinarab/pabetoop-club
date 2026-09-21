package app

import (
	"crypto/subtle"
	"net/http"

	"github.com/pocketbase/pocketbase/core"
)

const csrfCookieName = "pabetoop_csrf"

func csrfToken(re *core.RequestEvent) string {
	if cookie, err := re.Request.Cookie(csrfCookieName); err == nil && len(cookie.Value) >= 32 {
		return cookie.Value
	}
	token := secureToken() + secureToken()
	re.SetCookie(&http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60,
		Secure:   re.IsTLS(),
		SameSite: http.SameSiteLaxMode,
	})
	return token
}

func validateCSRF(re *core.RequestEvent) error {
	cookie, err := re.Request.Cookie(csrfCookieName)
	if err != nil || cookie.Value == "" {
		return re.ForbiddenError("درخواست امنیتی معتبر نیست.", nil)
	}
	provided := re.Request.FormValue("csrf_token")
	if subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(provided)) != 1 {
		return re.ForbiddenError("درخواست امنیتی معتبر نیست.", nil)
	}
	return nil
}

func hiddenCSRF(token string) string {
	return `<input type="hidden" name="csrf_token" value="` + escape(token) + `">`
}
