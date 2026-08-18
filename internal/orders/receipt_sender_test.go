package orders

import (
	"context"
	"testing"

	"github.com/example/checkout-receipt-service/internal/infrai"
)

type recordingSender struct {
	calls int
	email infrai.Email
	key   string
}

func (s *recordingSender) SendEmail(_ context.Context, email infrai.Email, key string) (infrai.SendResult, error) {
	s.calls++
	s.email = email
	s.key = key
	return infrai.SendResult{MessageID: "msg_test"}, nil
}

func TestOrderUpdateNotificationDecision(t *testing.T) {
	tests := []struct {
		name             string
		update           Update
		wantNotification string
		wantSubject      string
		wantKey          string
		wantCalls        int
	}{
		{
			name:             "paid checkout sends receipt",
			update:           Update{OrderID: "A-42", CustomerEmail: "buyer@example.com", Status: StatusPaid, TotalCents: 2599, Currency: "USD"},
			wantNotification: "receipt", wantSubject: "Receipt for order A-42", wantKey: "order-A-42-paid", wantCalls: 1,
		},
		{
			name:             "shipment sends tracking update",
			update:           Update{OrderID: "A-42", CustomerEmail: "buyer@example.com", Status: StatusShipped, TrackingCode: "TRACK-9"},
			wantNotification: "fulfillment_update", wantSubject: "Order A-42 has shipped", wantKey: "order-A-42-shipped", wantCalls: 1,
		},
		{
			name:             "processing does not notify",
			update:           Update{OrderID: "A-42", CustomerEmail: "buyer@example.com", Status: Status("processing")},
			wantNotification: "none", wantCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &recordingSender{}
			outcome, err := NewReceiptSender(recorder).Handle(context.Background(), tt.update)
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Notification != tt.wantNotification || recorder.calls != tt.wantCalls {
				t.Fatalf("outcome = %#v, calls = %d", outcome, recorder.calls)
			}
			if recorder.email.Subject != tt.wantSubject || recorder.key != tt.wantKey {
				t.Fatalf("subject = %q, key = %q", recorder.email.Subject, recorder.key)
			}
		})
	}
}
