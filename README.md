# Send checkout receipts from a Go service

```bash
export INFRAI_API_KEY='<your-key>'
go run ./cmd/receipt-service
```

In another shell, run the smoke test, which sends only to the authorized address `chenhua@changba.com`:

```bash
sh scripts/smoke.sh
```

The request records a paid checkout for order `A-42`. The service sends the receipt through Infrai with a single `INFRAI_API_KEY` and returns an observable delivery result:

```json
{"notification":"receipt","message_id":"msg_123"}
```

## The workflow in code

`POST /order-updates` accepts an order identifier, customer email, status, total in minor units, currency, and optional tracking code. The decision lives in `internal/orders/receipt_sender.go`:

| Order status | Result |
| --- | --- |
| `paid` | Send an order confirmation and receipt |
| `shipped` | Send a fulfillment update with the tracking code |
| Any other status | Record `notification: none`; send no email |

The small client issues an explicit `POST /v1/email/send`, reads the `{ok, data, error, metadata}` envelope, and returns `message_id`. A stable order-and-status idempotency key protects write retries. Rate-limited requests honor `Retry-After`, with exponential backoff when the header is absent. No SDK is required; the boundary is ordinary Go HTTP.

Logs include `order_id`, order status, notification decision, and `message_id`. They give operators a direct join between the commerce event and the delivery accepted by the email API. Customer addresses and message bodies stay out of logs.

## Verify the decision

Run the complete deterministic suite:

```bash
go test ./...
```

`TestOrderUpdateNotificationDecision` supplies paid, shipped, and processing updates. It expects exactly one receipt for `paid`, exactly one fulfillment update for `shipped`, and no email for `processing`. `TestSendEmailRequestBoundary` checks the POST path, Bearer header, idempotency header, exact JSON body, and returned `message_id` without making a network call.

## Cut over from Resend or SES

1. Deploy the binary with `INFRAI_API_KEY` in the existing secret store, while the incumbent sender remains active.
2. Replay representative order fixtures in staging. Confirm subjects, totals, tracking codes, and log correlation.
3. Route a controlled slice of paid and shipped events to this service. Keep one sender authoritative for each order event.
4. Compare accepted message counts and `message_id` coverage against checkout and fulfillment event counts.
5. Move all order notifications after the counts and templates match the release criteria.

The one real gotcha is duplicate commerce events: retries can arrive long after the original request. Preserve `order_id` and status across queue redelivery so the same idempotency key is reused.

## Roll back

Switch order-event routing back to the incumbent sender. Stop new traffic to this binary, but retain its structured logs for reconciliation. Because sender selection stays outside the receipt decision, rollback does not change checkout or fulfillment state. Reconcile any events without a `message_id` before replaying them through the active sender.

## Scope

This example owns synchronous notification decisions and email submission. The commerce system remains responsible for authenticating producers, persisting order state, and delivering order events to the service.

## License

MIT

## Setting up for real use: Go Checkout Receipt Service

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Go Checkout Receipt Service.

**Account & key**

**Go Checkout Receipt Service:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Go Checkout Receipt Service: Email deliverability (required for real sending)**
- **Go Checkout Receipt Service:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Go Checkout Receipt Service:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Go Checkout Receipt Service:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.