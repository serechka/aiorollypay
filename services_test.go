package rollypay

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPaymentsCreateBuildsCorrectRequest(t *testing.T) {
	var path string
	var body map[string]any
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_ = json.Unmarshal(readAll(t, r.Body), &body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"payment_id":"pay_1","status":"created","pay_url":"https://pay.rollypay.io/pay/tok_1"}`))
	}))

	pay, err := c.Payments.Create(context.Background(), CreatePaymentInput{
		Amount:             "1500.00",
		OrderID:            "order_12345",
		PaymentMethod:      PaymentMethodSBP,
		Description:        "Оплата заказа #12345",
		CustomerID:         "cust_001",
		SuccessRedirectURL: "https://myshop.com/success",
		FailRedirectURL:    "https://myshop.com/fail",
		Metadata:           map[string]any{"internal_ref": "INV-2026-0042"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if path != "/payments" {
		t.Errorf("path = %q, want /payments", path)
	}
	if pay.PaymentID != "pay_1" || pay.Status != PaymentStatusCreated {
		t.Errorf("payment = %+v", pay)
	}
	if body["amount"] != "1500.00" || body["order_id"] != "order_12345" {
		t.Errorf("body = %v", body)
	}
	if body["payment_currency"] != CurrencyRUB {
		t.Errorf("payment_currency = %v, want default RUB", body["payment_currency"])
	}
	if _, present := body["terminal_id"]; present {
		t.Error("terminal_id should be omitted when empty")
	}
	if _, present := body["test"]; present {
		t.Error("test should be omitted when false")
	}
	meta, ok := body["metadata"].(map[string]any)
	if !ok || meta["internal_ref"] != "INV-2026-0042" {
		t.Errorf("metadata = %v", body["metadata"])
	}
}

func TestPaymentsCreateValidation(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should be sent on validation failure")
	}))

	if _, err := c.Payments.Create(context.Background(), CreatePaymentInput{OrderID: "x"}); err == nil {
		t.Error("missing amount should fail")
	}
	if _, err := c.Payments.Create(context.Background(), CreatePaymentInput{Amount: "1"}); err == nil {
		t.Error("missing order_id should fail")
	}
}

func TestPaymentsCreateSandbox(t *testing.T) {
	var body map[string]any
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.Unmarshal(readAll(t, r.Body), &body)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"payment_id":"pay_1","test":true,"environment":"sandbox"}`))
	}))

	pay, err := c.Payments.Create(context.Background(), CreatePaymentInput{Amount: "5.00", OrderID: "t1", Test: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if body["test"] != true {
		t.Errorf("test flag = %v, want true", body["test"])
	}
	if !pay.Sandbox() {
		t.Error("Sandbox() = false, want true")
	}
}

func TestPaymentsGet(t *testing.T) {
	var path string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"payment_id":"pay_1","status":"paid","rate_final":"93.89","paid_at":"2026-02-23T12:05:32Z"}`))
	}))

	pay, err := c.Payments.Get(context.Background(), "pay_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if path != "/payments/pay_1" {
		t.Errorf("path = %q", path)
	}
	if pay.Status != PaymentStatusPaid || pay.RateFinal != "93.89" {
		t.Errorf("payment = %+v", pay)
	}
	if pay.PaidAt.IsZero() {
		t.Error("paid_at should be parsed")
	}
	if _, err := c.Payments.Get(context.Background(), ""); err == nil {
		t.Error("empty payment_id should fail")
	}
}

func TestPaymentsListQuery(t *testing.T) {
	var path, rawQuery string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, rawQuery = r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"items":[{"payment_id":"pay_1","status":"paid"}],"total":1}`))
	}))

	list, err := c.Payments.List(context.Background(), &PaymentsFilter{
		TerminalID:  "d290f1ee",
		Status:      PaymentStatusPaid,
		DateFrom:    time.Date(2026, 2, 23, 12, 0, 0, 0, time.UTC),
		Environment: EnvironmentSandbox,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if path != "/payments" {
		t.Errorf("path = %q", path)
	}
	for _, want := range []string{"terminal_id=d290f1ee", "status=paid", "from=2026-02-23T12%3A00%3A00Z", "environment=sandbox", "limit=20", "page=1"} {
		if !strings.Contains(rawQuery, want) {
			t.Errorf("rawQuery %q missing %q", rawQuery, want)
		}
	}
	if list.Total != 1 || len(list.Items) != 1 || list.Items[0].PaymentID != "pay_1" {
		t.Errorf("list = %+v", list)
	}
}

func TestPaymentsListDefaultsAndEnvelope(t *testing.T) {
	var rawQuery string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`[{"payment_id":"pay_2"}]`))
	}))

	list, err := c.Payments.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !strings.Contains(rawQuery, "limit=20") || !strings.Contains(rawQuery, "page=1") {
		t.Errorf("rawQuery = %q, want defaults", rawQuery)
	}
	if list.Total != 1 || len(list.Items) != 1 {
		t.Errorf("bare array decode: %+v", list)
	}
}

func TestPaymentsStatsQuery(t *testing.T) {
	var path, rawQuery string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, rawQuery = r.URL.Path, r.URL.RawQuery
		_, _ = w.Write([]byte(`{"count":42}`))
	}))

	out, err := c.Payments.Stats(context.Background(), &PaymentsFilter{Limit: 5, Page: 2, Status: PaymentStatusPaid})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if path != "/payments/stats" {
		t.Errorf("path = %q", path)
	}
	if strings.Contains(rawQuery, "limit=") || strings.Contains(rawQuery, "page=") {
		t.Errorf("rawQuery = %q, limit/page should be dropped", rawQuery)
	}
	if !strings.Contains(rawQuery, "status=paid") {
		t.Errorf("rawQuery = %q missing status", rawQuery)
	}
	if out["count"] != float64(42) {
		t.Errorf("stats = %v", out)
	}
}

func TestMiscEndpoints(t *testing.T) {
	var path, rawQuery string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, rawQuery = r.URL.Path, r.URL.RawQuery
		switch path {
		case "/me":
			_, _ = w.Write([]byte(`{"id":"usr_1","role":"merchant_owner"}`))
		case "/balance":
			_, _ = w.Write([]byte(`{"terminal_id":"t1","available_usdt":"1250.75","hold_usdt":"50.00"}`))
		case "/rate":
			_, _ = w.Write([]byte(`{"rate":"93.89","updated_at":"2026-02-23T12:00:00Z"}`))
		}
	}))

	me, err := c.Misc.Me(context.Background())
	if err != nil || me["id"] != "usr_1" {
		t.Errorf("Me = %v, err %v", me, err)
	}

	bal, err := c.Misc.Balance(context.Background(), "t1")
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if !strings.Contains(rawQuery, "terminal_id=t1") {
		t.Errorf("rawQuery = %q", rawQuery)
	}
	if bal.AvailableUSDT != "1250.75" || bal.HoldUSDT != "50.00" {
		t.Errorf("balance = %+v", bal)
	}

	rate, err := c.Misc.Rate(context.Background(), "")
	if err != nil {
		t.Fatalf("Rate: %v", err)
	}
	if rawQuery != "" {
		t.Errorf("rawQuery = %q, want empty for terminal-less rate", rawQuery)
	}
	if rate.Rate != "93.89" {
		t.Errorf("rate = %+v", rate)
	}
}

func TestTerminalsService(t *testing.T) {
	var method, path string
	var body map[string]any
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		if r.Body != nil {
			_ = json.Unmarshal(readAll(t, r.Body), &body)
		}
		switch {
		case path == "/terminals":
			_, _ = w.Write([]byte(`[{"id":"t1","name":"Касса","fee_payer":"client"}]`))
		case strings.HasSuffix(path, "/rotate-api-key"):
			_, _ = w.Write([]byte(`{"api_key":"rpk_live_new"}`))
		case strings.HasSuffix(path, "/providers"):
			_, _ = w.Write([]byte(`[{"name":"sbp"}]`))
		default:
			_, _ = w.Write([]byte(`{"id":"t1","name":"Касса","fee_payer":"client"}`))
		}
	}))

	list, err := c.Terminals.List(context.Background())
	if err != nil || len(list) != 1 || list[0].Name != "Касса" {
		t.Fatalf("List = %v, err %v", list, err)
	}

	term, err := c.Terminals.Get(context.Background(), "t1")
	if err != nil || term.ID != "t1" || term.Name != "Касса" {
		t.Errorf("Get = %+v, err %v", term, err)
	}

	no := false
	upd, err := c.Terminals.Update(context.Background(), "t1", TerminalUpdate{
		CallbackURL:          "https://myshop.com/webhooks/rollypay",
		ShowPlatformBranding: &no,
		FeePayer:             FeePayerMerchant,
	})
	if err != nil || upd.ID != "t1" {
		t.Errorf("Update = %+v, err %v", upd, err)
	}
	if method != "PUT" || path != "/terminals/t1" {
		t.Errorf("update call = %s %s", method, path)
	}
	if body["callback_url"] != "https://myshop.com/webhooks/rollypay" || body["fee_payer"] != "merchant" {
		t.Errorf("update body = %v", body)
	}
	if _, present := body["name"]; present {
		t.Error("zero fields must be omitted from update body")
	}
	if body["show_platform_branding"] != false {
		t.Errorf("explicit false pointer lost: %v", body["show_platform_branding"])
	}

	key, err := c.Terminals.RotateAPIKey(context.Background(), "t1")
	if err != nil || key.APIKey != "rpk_live_new" {
		t.Errorf("RotateAPIKey = %+v, err %v", key, err)
	}

	prov, err := c.Terminals.Providers(context.Background(), "t1")
	if err != nil || len(prov) != 1 || prov[0]["name"] != "sbp" {
		t.Errorf("Providers = %v, err %v", prov, err)
	}
}

func TestTerminalsValidation(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}))
	if _, err := c.Terminals.Get(context.Background(), ""); err == nil {
		t.Error("Get requires terminal_id")
	}
	if _, err := c.Terminals.Update(context.Background(), "", TerminalUpdate{}); err == nil {
		t.Error("Update requires terminal_id")
	}
	if _, err := c.Terminals.RotateAPIKey(context.Background(), ""); err == nil {
		t.Error("RotateAPIKey requires terminal_id")
	}
	if _, err := c.Terminals.Providers(context.Background(), ""); err == nil {
		t.Error("Providers requires terminal_id")
	}
}

func TestStatsService(t *testing.T) {
	var rawQuery string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"volume":"100.00"}`))
	}))

	over, err := c.Stats.Overview(context.Background(), "t1")
	if err != nil || over["volume"] != "100.00" {
		t.Errorf("Overview = %v, err %v", over, err)
	}
	if !strings.Contains(rawQuery, "terminal_id=t1") {
		t.Errorf("rawQuery = %q", rawQuery)
	}

	sum, err := c.Stats.FinanceSummary(context.Background(), "")
	if err != nil {
		t.Fatalf("FinanceSummary: %v", err)
	}
	if sum == nil {
		t.Error("FinanceSummary returned nil")
	}

	ts, err := c.Stats.Timeseries(context.Background(), "volume", "day", "t1")
	if err != nil || ts == nil {
		t.Errorf("Timeseries = %v, err %v", ts, err)
	}
	if !strings.Contains(rawQuery, "metric=volume") || !strings.Contains(rawQuery, "interval=day") {
		t.Errorf("rawQuery = %q", rawQuery)
	}

	if _, err := c.Stats.Overview(context.Background(), ""); err == nil {
		t.Error("Overview requires terminal_id")
	}
	if _, err := c.Stats.Timeseries(context.Background(), "", "day", ""); err == nil {
		t.Error("Timeseries requires metric")
	}
	if _, err := c.Stats.Timeseries(context.Background(), "volume", "", ""); err == nil {
		t.Error("Timeseries requires interval")
	}
}

func TestPayoutsService(t *testing.T) {
	var method, path, rawQuery string
	var body map[string]any
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, rawQuery = r.Method, r.URL.Path, r.URL.RawQuery
		if r.Body != nil {
			_ = json.Unmarshal(readAll(t, r.Body), &body)
		}
		if path == "/payouts" && method == "POST" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"po_1","status":"created","amount_usdt":"50.00"}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"po_1"},{"id":"po_2"}],"total":2}`))
	}))

	pay, err := c.Payouts.Create(context.Background(), CreatePayoutInput{
		AmountUSDT:     "50.00",
		WalletAddress:  "TJYkxBf0XG3P8nFHv1i9zNbeQ7oCF3qLzR",
		Network:        "TRC-20",
		IdempotencyKey: "payout_20260223_001",
		TerminalID:     "t1",
	})
	if err != nil || pay.ID != "po_1" || pay.Status != PayoutStatusCreated {
		t.Errorf("Create = %+v, err %v", pay, err)
	}
	if rawQuery != "terminal_id=t1" {
		t.Errorf("rawQuery = %q", rawQuery)
	}
	if body["amount_usdt"] != "50.00" || body["network"] != "TRC-20" || body["idempotency_key"] != "payout_20260223_001" {
		t.Errorf("body = %v", body)
	}
	if _, present := body["terminal_id"]; present {
		t.Error("terminal_id must go to query, not body")
	}

	list, err := c.Payouts.List(context.Background(), "")
	if err != nil || list.Total != 2 || len(list.Items) != 2 {
		t.Errorf("List = %+v, err %v", list, err)
	}
	if method != "GET" {
		t.Errorf("list method = %s", method)
	}
}

func TestPayoutsCreateValidation(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}))
	base := CreatePayoutInput{AmountUSDT: "1", WalletAddress: "T...", Network: "TRC-20"}
	if _, err := c.Payouts.Create(context.Background(), CreatePayoutInput{}); err == nil {
		t.Error("empty input should fail")
	}
	if _, err := c.Payouts.Create(context.Background(), CreatePayoutInput{WalletAddress: "T...", Network: "TRC-20"}); err == nil {
		t.Error("missing amount should fail")
	}
	if _, err := c.Payouts.Create(context.Background(), CreatePayoutInput{AmountUSDT: "1", Network: "TRC-20"}); err == nil {
		t.Error("missing wallet should fail")
	}
	if _, err := c.Payouts.Create(context.Background(), CreatePayoutInput{AmountUSDT: "1", WalletAddress: "T..."}); err == nil {
		t.Error("missing network should fail")
	}
	_ = base
}

func TestSubscriptionsFlow(t *testing.T) {
	var method, rawQuery string
	var body map[string]any
	var idemKey string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, rawQuery = r.Method, r.URL.RawQuery
		path := r.URL.Path
		idemKey = r.Header.Get("Idempotency-Key")
		if r.Body != nil {
			_ = json.Unmarshal(readAll(t, r.Body), &body)
		}
		switch {
		case path == "/subscription-plans":
			_, _ = w.Write([]byte(`[{"id":"plan_1","interval":"month","payer_amount_rub":"150.00"}]`))
		case path == "/subscriptions" && method == "POST":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"sub_1","state":"new","billing_status":"consent_pending","pay_url":"https://pay.rollypay.io/pay/tok_9"}`))
		case strings.HasSuffix(path, "/charges"):
			_, _ = w.Write([]byte(`[{"payment_id":"pay_9","cycle_number":1,"status":"payed"}]`))
		case strings.HasSuffix(path, "/stop"):
			_, _ = w.Write([]byte(`{"ok":true}`))
		case path == "/subscriptions":
			_, _ = w.Write([]byte(`{"items":[{"id":"sub_1"}],"total":1}`))
		default:
			_, _ = w.Write([]byte(`{"id":"sub_1","state":"active"}`))
		}
	}))

	plans, err := c.Subscriptions.Plans(context.Background(), "t1")
	if err != nil || len(plans) != 1 || plans[0].PayerAmountRUB != "150.00" {
		t.Errorf("Plans = %v, err %v", plans, err)
	}

	sub, err := c.Subscriptions.Create(context.Background(), CreateSubscriptionInput{
		TerminalID:     "t1",
		PlanID:         "plan_1",
		PayerID:        "tg_123456789",
		MerchantRef:    "customer-123-vpn",
		IdempotencyKey: "customer-123-plan-30d-attempt-1",
	})
	if err != nil || sub.ID != "sub_1" || sub.State != SubscriptionStateNew {
		t.Errorf("Create = %+v, err %v", sub, err)
	}
	if idemKey != "customer-123-plan-30d-attempt-1" {
		t.Errorf("Idempotency-Key = %q", idemKey)
	}
	if body["payer_id"] != "tg_123456789" {
		t.Errorf("body = %v", body)
	}
	if _, present := body["amount"]; present {
		t.Error("amount must be omitted for fixed plans")
	}

	got, err := c.Subscriptions.Get(context.Background(), "sub_1")
	if err != nil || got.State != SubscriptionStateActive {
		t.Errorf("Get = %+v, err %v", got, err)
	}

	list, err := c.Subscriptions.List(context.Background(), "t1", 0)
	if err != nil || list.Total != 1 {
		t.Errorf("List = %+v, err %v", list, err)
	}
	if !strings.Contains(rawQuery, "page=1") {
		t.Errorf("rawQuery = %q, want default page 1", rawQuery)
	}

	charges, err := c.Subscriptions.Charges(context.Background(), "sub_1")
	if err != nil || len(charges) != 1 || charges[0].Status != ChargeStatusPayed {
		t.Errorf("Charges = %v, err %v", charges, err)
	}

	stop, err := c.Subscriptions.Stop(context.Background(), "sub_1")
	if err != nil || !stop.OK {
		t.Errorf("Stop = %+v, err %v", stop, err)
	}
}

func TestSubscriptionsValidation(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request expected")
	}))
	if _, err := c.Subscriptions.Plans(context.Background(), ""); err == nil {
		t.Error("Plans requires terminal_id")
	}
	if _, err := c.Subscriptions.Create(context.Background(), CreateSubscriptionInput{PlanID: "p"}); err == nil {
		t.Error("Create requires terminal_id")
	}
	if _, err := c.Subscriptions.Create(context.Background(), CreateSubscriptionInput{TerminalID: "t"}); err == nil {
		t.Error("Create requires plan_id")
	}
	if _, err := c.Subscriptions.Create(context.Background(), CreateSubscriptionInput{
		TerminalID: "t", PlanID: "p", PayerID: strings.Repeat("x", 33),
	}); err == nil {
		t.Error("payer_id over 32 bytes should fail")
	}
	if _, err := c.Subscriptions.Create(context.Background(), CreateSubscriptionInput{
		TerminalID: "t", PlanID: "p", MerchantRef: strings.Repeat("x", 129),
	}); err == nil {
		t.Error("merchant_subscription_ref over 128 bytes should fail")
	}
	if _, err := c.Subscriptions.Create(context.Background(), CreateSubscriptionInput{
		TerminalID: "t", PlanID: "p", PayerID: "a", Amount: "990.00",
	}); err == nil {
		t.Error("payer_id and amount are mutually exclusive")
	}
	if _, err := c.Subscriptions.Get(context.Background(), ""); err == nil {
		t.Error("Get requires id")
	}
	if _, err := c.Subscriptions.List(context.Background(), "", 1); err == nil {
		t.Error("List requires terminal_id")
	}
	if _, err := c.Subscriptions.Charges(context.Background(), ""); err == nil {
		t.Error("Charges requires id")
	}
	if _, err := c.Subscriptions.Stop(context.Background(), ""); err == nil {
		t.Error("Stop requires id")
	}
}

func TestSubscriptionsAutoIdempotencyKey(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Idempotency-Key") == "" {
			t.Error("auto idempotency key missing")
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"sub_1"}`))
	}))
	if _, err := c.Subscriptions.Create(context.Background(), CreateSubscriptionInput{TerminalID: "t", PlanID: "p", PayerID: "a"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
}
