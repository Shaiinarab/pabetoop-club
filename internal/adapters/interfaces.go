// Package adapters abstracts external services behind interfaces so the app
// never depends on a live gateway. In sandbox STUB=1 the adapters return
// deterministic doubles; production injects real implementations via env.
package adapters

import (
	"context"
	"fmt"
	"os"
)

// PaymentGateway charges/receives money.
type PaymentGateway interface {
	Charge(ctx context.Context, playerID, amount int64, authority string) (string, error)
	Verify(ctx context.Context, authority string) (bool, error)
	Refund(ctx context.Context, authority string) error
}

// SMSService sends messages (OTP, reminders, receipts).
type SMSService interface {
	SendOTP(ctx context.Context, phone, code string) error
	SendReceipt(ctx context.Context, phone, ref string, amount int64) error
	SendReminder(ctx context.Context, phone, dueDate string, amount int64) error
}

// GpsIngestor ingests training-load data.
type GpsIngestor interface {
	IngestCSV(ctx context.Context, playerID int64, csv []byte) (int, error)
	IngestWebhook(ctx context.Context, payload []byte, secret string) (int, error)
}

// IsStub returns true when no live credentials are configured.
func IsStub() bool {
	return os.Getenv("STUB") != "0"
}

// StubCharge returns a deterministic receipt ref for sandbox testing.
func StubCharge(playerID int64, amount int64, authority string) string {
	return fmt.Sprintf("STUB-%d-%d-%s", playerID, amount, authority)
}
