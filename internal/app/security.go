package app

import "github.com/pocketbase/pocketbase/core"

func securityHeaders(re *core.RequestEvent) error {
	headers := re.Response.Header()
	headers.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; script-src 'self' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'")
	headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	headers.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	headers.Set("Cross-Origin-Opener-Policy", "same-origin")
	headers.Set("X-Content-Type-Options", "nosniff")
	return re.Next()
}
