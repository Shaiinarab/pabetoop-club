// Package adapters provides stub (sandbox-safe) implementations of external
// services. Real implementations (ZarinPal, IDPay, Kavenegar, STATSports) live
// alongside these and are selected via env flags in production.
package adapters

import (
	"context"
	"log"
	"os"
)

// StubPayment is a no-network payment adapter for the sandbox.
type StubPayment struct{}

func NewStubPayment() *StubPayment { return &StubPayment{} }

func (s *StubPayment) Charge(ctx context.Context, playerID, amount int64, authority string) (string, error) {
	if !IsStub() {
		log.Println("[payment] WARN: stub called in non-stub mode")
	}
	ref := StubCharge(playerID, amount, authority)
	log.Printf("[STUB] payment charge player=%d amount=%d authority=%s ref=%s", playerID, amount, authority, ref)
	return ref, nil
}

func (s *StubPayment) Verify(ctx context.Context, authority string) (bool, error) {
	return true, nil
}

func (s *StubPayment) Refund(ctx context.Context, authority string) error {
	log.Printf("[STUB] payment refund authority=%s", authority)
	return nil
}

// StubSMS is a no-network SMS adapter for the sandbox.
type StubSMS struct{}

func NewStubSMS() *StubSMS { return &StubSMS{} }

func (s *StubSMS) SendOTP(ctx context.Context, phone, code string) error {
	if !IsStub() {
		log.Println("[sms] WARN: stub called in non-stub mode")
	}
	log.Printf("[STUB] sms otp phone=%s code=%s origin=%s", phone, code, os.Getenv("SMS_ORIGIN"))
	return nil
}

func (s *StubSMS) SendReceipt(ctx context.Context, phone, ref string, amount int64) error {
	log.Printf("[STUB] sms receipt phone=%s ref=%s amount=%d origin=%s", phone, ref, amount, os.Getenv("SMS_ORIGIN"))
	return nil
}

func (s *StubSMS) SendReminder(ctx context.Context, phone, dueDate string, amount int64) error {
	log.Printf("[STUB] sms reminder phone=%s due=%s amount=%d origin=%s", phone, dueDate, amount, os.Getenv("SMS_ORIGIN"))
	return nil
}

// StubGps is a deterministic GPS ingestor for the sandbox.
type StubGps struct{}

func NewStubGps() *StubGps { return &StubGps{} }

func (s *StubGps) IngestCSV(ctx context.Context, playerID int64, csv []byte) (int, error) {
	if !IsStub() {
		log.Println("[gps] WARN: stub called in non-stub mode")
	}
	log.Printf("[STUB] gps csv ingest player=%d bytes=%d", playerID, len(csv))
	return 0, nil
}

func (s *StubGps) IngestWebhook(ctx context.Context, payload []byte, secret string) (int, error) {
	log.Printf("[STUB] gps webhook ingest bytes=%d secret=%s", len(payload), secret)
	return 0, nil
}
