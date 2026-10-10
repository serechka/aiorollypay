// Command parallel shows the "asynchronous" Go pattern: the client is
// thread-safe, so independent API calls fan out across goroutines and the
// total wall time is the slowest call, not the sum.
//
// Usage: ROLLYPAY_API_KEY=rpk_live_... go run ./examples/parallel
package main

import (
	"context"
	"log"
	"os"
	"sync"
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	start := time.Now()

	var (
		wg       sync.WaitGroup
		errCh    = make(chan error, 3)
		balance  *rollypay.Balance
		rate     *rollypay.Rate
		payments *rollypay.PaymentList
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		v, err := client.Misc.Balance(ctx, "")
		if err != nil {
			errCh <- err
			return
		}
		balance = v
	}()
	go func() {
		defer wg.Done()
		v, err := client.Misc.Rate(ctx, "")
		if err != nil {
			errCh <- err
			return
		}
		rate = v
	}()
	go func() {
		defer wg.Done()
		v, err := client.Payments.List(ctx, &rollypay.PaymentsFilter{Limit: 5})
		if err != nil {
			errCh <- err
			return
		}
		payments = v
	}()

	wg.Wait()
	close(errCh)
	for err := range errCh {
		log.Printf("вызов не удался: %v", err)
	}

	if balance != nil {
		log.Printf("баланс: %s USDT (hold %s)", balance.AvailableUSDT, balance.HoldUSDT)
	}
	if rate != nil {
		log.Printf("курс: %s RUB/USDT", rate.Rate)
	}
	if payments != nil {
		log.Printf("последние платежи: %d из %d", len(payments.Items), payments.Total)
	}
	log.Printf("готово за %v (три вызова параллельно)", time.Since(start))
}
