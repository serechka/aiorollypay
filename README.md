# aiorollypay

Клиент платёжной системы [RollyPay](https://rollypay.io) для Go. Покрывает публичное API [docs.rollypay.io](https://docs.rollypay.io): платежи, кассы, выплаты, рекуррентные подписи СБП, статистику, баланс, курс и вебхуки с проверкой подписи.

Официального Go SDK у RollyPay нет. Контракт сверен с официальным [Python SDK](https://github.com/RlyPy/sdk-python) и документацией API.

Зависимостей нет: только стандартная библиотека (`net/http`, `crypto/hmac`, `crypto/rand`).

## Требования

Go 1.22 или новее.

## Установка

```bash
go get github.com/serechka/aiorollypay
```

## Использование

```go
import "github.com/serechka/aiorollypay" // пакет rollypay

client, err := rollypay.New("rpk_live_ключ")
if err != nil {
	log.Fatal(err)
}

payment, err := client.Payments.Create(ctx, rollypay.CreatePaymentInput{
	Amount:             "1500.00",
	OrderID:            "order_12345",
	PaymentMethod:      rollypay.PaymentMethodSBP,
	Description:        "Оплата заказа #12345",
	SuccessRedirectURL: "https://myshop.com/success",
	FailRedirectURL:    "https://myshop.com/fail",
	Metadata:           map[string]any{"internal_ref": "INV-2026-0042"},
})
if err != nil {
	log.Fatal(err)
}

// перенаправить клиента на payment.PayURL
```

Суммы передаются строками, как в API: без погрешностей float.

## Методы

| Сервис | Методы |
|---|---|
| `client.Payments` | `Create`, `Get`, `List`, `Stats` |
| `client.Terminals` | `List`, `Get`, `Update`, `RotateAPIKey`, `Providers` |
| `client.Payouts` | `Create`, `List` |
| `client.Subscriptions` | `Plans`, `Create`, `Get`, `List`, `Charges`, `Stop` |
| `client.Stats` | `Overview`, `FinanceSummary`, `Timeseries` |
| `client.Misc` | `Me`, `Balance`, `Rate` |
| `webhooks` | `Sign`, `VerifySignature`, `ParseEvent`, `Handler` |

Каждый метод принимает `context.Context` первым аргументом. `Client` потокобезопасен: создаётся один раз и разделяется между горутинами.

## Вебхуки

RollyPay подписывает колбэки: `X-Signature` содержит hex HMAC-SHA256 от строки `timestamp + "." + body`, ключом служит `signing_secret` кассы. Подпись проверяется до обработки payload.

```go
import "github.com/serechka/aiorollypay/webhooks"

http.Handle("/webhooks/rollypay", webhooks.Handler(signingSecret,
	func(ctx context.Context, e *webhooks.Event) error {
		if e.Sandbox() {
			return nil
		}
		switch e.EventType {
		case webhooks.EventPaymentPaid:
			// выдать доступ один раз на payment_id
		}
		return nil
	}))
```

`Handler` возвращает 403 при неверной подписи, 400 при некорректном payload и 500 при ошибке пользовательской функции; на 500 RollyPay повторит доставку (до 8 попыток с backoff). Доставка минимум однократная, статусы платежей меняются и задним числом: `expired` может стать `paid`, `paid` стать `refunded`. Обработчик обязан быть идемпотентным.

## Sandbox

Тестовый режим включается на клиенте (`rollypay.WithTestMode(true)`) или на отдельном вызове (`Test: true` в `CreatePaymentInput`). Результат симулируется кнопками на тестовой `pay_url`. Тестовые колбэки помечены полем `test` и заголовком `X-Test-Mode`.

## Ошибки

Любая ошибка клиента имеет тип `*rollypay.Error`; проверяется через `errors.As`, для типовых кодов есть предикаты:

```go
switch {
case rollypay.IsNotFound(err):       // 404
case rollypay.IsAuthentication(err): // 401: ключ или nonce
case rollypay.IsRateLimited(err):    // 429: в Error.RetryAfter подсказка сервера
case rollypay.IsConflict(err):       // 409
case rollypay.IsServer(err):         // 5xx
case rollypay.IsTransport(err):      // сеть или таймаут, StatusCode == 0
}
```

Текст ошибки берётся из поля `error` ответа API.

## Таймауты и повторы

- Без дедлайна в контексте действует таймаут попытки: 30 с по умолчанию, меняется `WithTimeout`.
- GET повторяется при 429, 5xx и сетевых сбоях: 2 повтора по умолчанию, настраивается `WithRetry(maxAttempts, backoff)`. Пауза берётся из `Retry-After` либо удваивается от `backoff`.
- POST и PUT не повторяются. Идемпотентность остаётся на вызывающем: `order_id` у платежей, `IdempotencyKey` у подписок, `idempotency_key` у выплат.
- Каждый запрос несёт `X-API-Key` и новый `X-Nonce` (UUID v4); API отклоняет повтор nonce в 10-минутном окне.

## Производительность

Замеры `go test -bench . -benchmem` (Go 1.26, localhost, верхняя граница накладных расходов):

| Бенчмарк | Время | Память | Аллокации |
|---|---|---|---|
| Вызов API, полный цикл | 75 мкс | 8.5 КБ | 101 |
| Сериализация `CreatePaymentInput` | 540 нс | 496 Б | 5 |
| Проверка подписи вебхука | 670 нс | 848 Б | 12 |
| Генерация `X-Nonce` | 107 нс | 48 Б | 1 |

## Примеры

В [examples/](examples/): создание платежа, параллельные запросы, приём вебхуков.

## Разработка

```bash
go build ./... && go vet ./... && go test ./... -race -cover
go test -run=NONE -bench=. -benchmem
```

## Лицензия

[MIT](LICENSE)
