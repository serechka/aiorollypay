// Command webhook-server runs an HTTPS-ready HTTP endpoint that receives
// RollyPay callbacks, verifies their HMAC signature and grants access exactly
// once per payment.
//
// Usage:
//
//	ROLLYPAY_SIGNING_SECRET=... go run ./examples/webhook-server
//	# then set https://<public-url>/webhooks/rollypay as the terminal callback_url
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/serechka/aiorollypay/webhooks"
)

func main() {
	secret := os.Getenv("ROLLYPAY_SIGNING_SECRET")
	if secret == "" {
		log.Fatal("set ROLLYPAY_SIGNING_SECRET (terminal signing_secret, not the API key)")
	}

	// Webhooks are delivered at-least-once and statuses are not permanently
	// final, so real handlers must be idempotent. Here: one goroutine-safe
	// set of processed payment IDs stands in for your database.
	var (
		mu       sync.Mutex
		verified = map[string]bool{}
	)

	http.Handle("/webhooks/rollypay", webhooks.Handler(secret, func(ctx context.Context, e *webhooks.Event) error {
		if e.Sandbox() {
			log.Printf("[тест] %s %s %s %s", e.EventType, e.PaymentID, e.Amount, e.Currency)
		} else {
			log.Printf("%s %s %s %s", e.EventType, e.PaymentID, e.Amount, e.Currency)
		}

		mu.Lock()
		defer mu.Unlock()
		if verified[e.PaymentID+e.EventType] {
			return nil // повторная доставка, уже обработано
		}
		verified[e.PaymentID+e.EventType] = true

		switch e.EventType {
		case webhooks.EventPaymentPaid:
			log.Printf("заказ %s оплачен: выдать доступ (один раз)", e.OrderID)
		case webhooks.EventPaymentExpired, webhooks.EventPaymentCanceled:
			log.Printf("заказ %s не оплачен (%s)", e.OrderID, e.Status)
		default:
			log.Printf("событие %s зафиксировано", e.EventType)
		}
		return nil
	}))

	addr := "127.0.0.1:8080"
	log.Printf("слушаю http://%s/webhooks/rollypay", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
