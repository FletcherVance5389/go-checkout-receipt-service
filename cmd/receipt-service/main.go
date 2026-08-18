package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/example/checkout-receipt-service/internal/infrai"
	"github.com/example/checkout-receipt-service/internal/orders"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	address := os.Getenv("LISTEN_ADDR")
	if address == "" {
		address = ":8080"
	}

	sender := orders.NewReceiptSender(infrai.NewClient(apiKey))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /order-updates", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var update orders.Update
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&update); err != nil {
			http.Error(w, "invalid order update", http.StatusBadRequest)
			return
		}
		outcome, err := sender.Handle(r.Context(), update)
		if err != nil {
			log.Printf("order_id=%q notification_error=%q", update.OrderID, err)
			http.Error(w, "notification delivery failed", http.StatusBadGateway)
			return
		}
		log.Printf("order_id=%q status=%q notification=%q message_id=%q", update.OrderID, update.Status, outcome.Notification, outcome.MessageID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(outcome)
	})

	server := &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("receipt service listening on %s", address)
	log.Fatal(server.ListenAndServe())
}
