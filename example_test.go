package rollypay_test

import (
	"context"
	"fmt"
	"log"

	rollypay "github.com/serechka/aiorollypay"
)

// Creating a payment is the most common integration flow.
func ExampleNew() {
	client, err := rollypay.New("rpk_live_example")
	if err != nil {
		log.Fatal(err)
	}

	payment, err := client.Payments.Create(context.Background(), rollypay.CreatePaymentInput{
		Amount:        "1500.00",
		OrderID:       "order_12345",
		PaymentMethod: rollypay.PaymentMethodSBP,
		Description:   "Оплата заказа #12345",
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(payment.PaymentID, payment.Status, payment.PayURL)
}
