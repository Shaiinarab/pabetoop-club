package app

import (
	"strings"
	"testing"
)

func TestPageShellIncludesPWAFoundation(t *testing.T) {
	html := pageShell("آزمون", "مدیر", "portal", "<p>محتوا</p>")
	for _, expected := range []string{
		`rel="manifest" href="/assets/manifest.webmanifest"`,
		`name="theme-color" content="#164b38"`,
		`src="/assets/js/pwa-register.js"`,
		`href="/assets/icons/icon-192.svg"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("page shell is missing PWA component %q", expected)
		}
	}
}
