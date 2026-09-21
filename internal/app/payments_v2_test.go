package app

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestTestGatewayVerification(t *testing.T) {
	gateway := testGateway{}
	started, err := gateway.Start(context.Background(), paymentStartRequest{AmountRial: 15000000})
	if err != nil {
		t.Fatalf("start test payment: %v", err)
	}
	if !strings.HasPrefix(started.Authority, "TST-") {
		t.Fatalf("test authority = %q; expected TST prefix", started.Authority)
	}
	verified, err := gateway.Verify(context.Background(), started.Authority, 15000000)
	if err != nil {
		t.Fatalf("verify test payment: %v", err)
	}
	if !strings.HasPrefix(verified.Reference, "TEST-TST-") {
		t.Fatalf("test reference = %q; expected TEST-TST prefix", verified.Reference)
	}
}

func TestTestGatewayRejectsForeignAuthority(t *testing.T) {
	if _, err := (testGateway{}).Verify(context.Background(), "A123", 15000000); err == nil {
		t.Fatal("foreign authority must be rejected by the deterministic test gateway")
	}
}

func TestCallbackURLUsesConfiguredBase(t *testing.T) {
	previous := os.Getenv("ZARINPAL_CALLBACK_BASE_URL")
	t.Cleanup(func() { _ = os.Setenv("ZARINPAL_CALLBACK_BASE_URL", previous) })
	if err := os.Setenv("ZARINPAL_CALLBACK_BASE_URL", "https://academy.example/"); err != nil {
		t.Fatal(err)
	}
	got := callbackURLForPayment("payment123")
	want := "https://academy.example/payments/callback?payment_id=payment123"
	if got != want {
		t.Fatalf("callback URL = %q; want %q", got, want)
	}
}
