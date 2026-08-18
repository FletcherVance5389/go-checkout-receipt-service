package infrai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSendEmailRequestBoundary(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/email/send" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("missing bearer authorization")
		}
		if r.Header.Get("Idempotency-Key") != "receipt-order-42-paid" {
			t.Fatal("missing stable idempotency key")
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"message_id":"msg_42"},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := newClient(server.URL, "test-key", server.Client())
	result, err := client.SendEmail(context.Background(), Email{
		To: "buyer@example.com", Subject: "Receipt for order 42", HTML: "<p>Paid</p>",
	}, "receipt-order-42-paid")
	if err != nil {
		t.Fatal(err)
	}
	if result.MessageID != "msg_42" {
		t.Fatalf("message_id = %q", result.MessageID)
	}
	want := map[string]any{"to": "buyer@example.com", "subject": "Receipt for order 42", "html": "<p>Paid</p>"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("body = %#v, want %#v", got, want)
	}
}
