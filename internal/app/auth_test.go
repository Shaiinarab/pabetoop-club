package app

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func TestNormalizeMobile(t *testing.T) {
	tests := map[string]string{
		"09171234567":     "09171234567",
		"0917 123 4567":   "09171234567",
		"+989171234567":   "09171234567",
		"989171234567":    "09171234567",
		"(0917)-123-4567": "09171234567",
	}

	for input, want := range tests {
		if got := normalizeMobile(input); got != want {
			t.Errorf("normalizeMobile(%q) = %q; want %q", input, got, want)
		}
	}
}

func TestLandingPath(t *testing.T) {
	staff := core.NewAuthCollection("staff")
	manager := core.NewRecord(staff)
	manager.Set("role", "manager")

	if got := landingPath(manager); got != "/manager" {
		t.Fatalf("manager landing path = %q; want /manager", got)
	}

	guardians := core.NewAuthCollection("guardians")
	guardian := core.NewRecord(guardians)
	if got := landingPath(guardian); got != "/portal" {
		t.Fatalf("guardian landing path = %q; want /portal", got)
	}
}

func TestIsManagerRejectsGuardian(t *testing.T) {
	guardian := core.NewRecord(core.NewAuthCollection("guardians"))
	if isManager(guardian) {
		t.Fatal("guardian must not receive manager access")
	}
}

func TestFixedWindowLimiter(t *testing.T) {
	limiter := newFixedWindowLimiter(2, time.Minute)
	now := time.Date(2026, time.August, 14, 10, 0, 0, 0, time.UTC)
	if !limiter.allow("127.0.0.1", now) || !limiter.allow("127.0.0.1", now.Add(time.Second)) {
		t.Fatal("first two attempts should be allowed")
	}
	if limiter.allow("127.0.0.1", now.Add(2*time.Second)) {
		t.Fatal("third attempt in the window should be denied")
	}
	if !limiter.allow("127.0.0.1", now.Add(time.Minute)) {
		t.Fatal("attempt should be allowed after the window resets")
	}
}
