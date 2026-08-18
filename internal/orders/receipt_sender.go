package orders

import (
	"context"
	"fmt"
	"html"

	"github.com/example/checkout-receipt-service/internal/infrai"
)

type Status string

const (
	StatusPaid    Status = "paid"
	StatusShipped Status = "shipped"
)

type Update struct {
	OrderID       string `json:"order_id"`
	CustomerEmail string `json:"customer_email"`
	Status        Status `json:"status"`
	TotalCents    int64  `json:"total_cents"`
	Currency      string `json:"currency"`
	TrackingCode  string `json:"tracking_code"`
}

type EmailSender interface {
	SendEmail(context.Context, infrai.Email, string) (infrai.SendResult, error)
}

type ReceiptSender struct {
	email EmailSender
}

func NewReceiptSender(email EmailSender) *ReceiptSender {
	return &ReceiptSender{email: email}
}

type Outcome struct {
	Notification string `json:"notification"`
	MessageID    string `json:"message_id,omitempty"`
}

func (s *ReceiptSender) Handle(ctx context.Context, update Update) (Outcome, error) {
	email, notification, send := notificationFor(update)
	if !send {
		return Outcome{Notification: "none"}, nil
	}
	result, err := s.email.SendEmail(ctx, email, "order-"+update.OrderID+"-"+string(update.Status))
	if err != nil {
		return Outcome{}, err
	}
	return Outcome{Notification: notification, MessageID: result.MessageID}, nil
}

func notificationFor(update Update) (infrai.Email, string, bool) {
	orderID := html.EscapeString(update.OrderID)
	switch update.Status {
	case StatusPaid:
		amount := fmt.Sprintf("%s %.2f", html.EscapeString(update.Currency), float64(update.TotalCents)/100)
		return infrai.Email{
			To:      update.CustomerEmail,
			Subject: "Receipt for order " + update.OrderID,
			HTML:    "<h1>Payment received</h1><p>Order " + orderID + " is confirmed.</p><p>Total: " + amount + "</p>",
		}, "receipt", true
	case StatusShipped:
		tracking := html.EscapeString(update.TrackingCode)
		return infrai.Email{
			To:      update.CustomerEmail,
			Subject: "Order " + update.OrderID + " has shipped",
			HTML:    "<h1>Your order is on the way</h1><p>Order " + orderID + " shipped.</p><p>Tracking: " + tracking + "</p>",
		}, "fulfillment_update", true
	default:
		return infrai.Email{}, "", false
	}
}
