// Command create-payment shows the minimal payment creation flow:
// create → redirect the customer to pay_url → poll the status.
//
// Usage: ROLLYPAY_API_KEY=rpk_live_... go run ./examples/create-payment
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/serechka/aiorollypay"
)

func main() {
	apiKey := os.Getenv("ROLLYPAY_API_KEY")
	if apiKey == "" {
		log.Fatal("set ROLLYPAY_API_KEY (see your terminal settings)")
	}

	client, err := rollypay.New(apiKey)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	payment, err := client.Payments.Create(ctx, rollypay.CreatePaymentInput{
		Amount:             "1500.00",
		OrderID:            fmt.Sprintf("order_%d", time.Now().UnixMilli()),
		PaymentMethod:      rollypay.PaymentMethodSBP,
		Description:        "Оплата заказа #12345",
		CustomerID:         "user@example.com",
		SuccessRedirectURL: "https://myshop.com/payment/success",
		FailRedirectURL:    "https://myshop.com/payment/fail",
		Metadata:           map[string]any{"internal_ref": "INV-2026-0042"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Платёж создан: %s\n  статус:  %s\n  ссылка:  %s\n",
		payment.PaymentID, payment.Status, payment.PayURL)
	fmt.Println("Перенаправьте клиента на pay_url и подтверждайте оплату вебхуком;")

	fetched, err := client.Payments.Get(ctx, payment.PaymentID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Текущий статус: %s\n", fetched.Status)
}
