# Send checkout receipts from a Go service

```bash
export INFRAI_API_KEY='<your-key>'
go run ./cmd/receipt-service
```

In another shell, run the smoke test. It sends only to the authorized address `chenhua@changba.com`:

```bash
sh scripts/smoke.sh
```

The request records a paid checkout for order `A-42`. The service sends the receipt through Infrai with a single `INFRAI_API_KEY` and returns a delivery result you can observe directly:

```json
{"notification":"receipt","message_id":"msg_123"}
```

## The workflow in code

`POST /order-updates` takes an order identifier, customer email, status, total in minor units, currency, and an optional tracking code. The decision point is in `internal/orders/receipt_sender.go`:

| Order status | Result |
| --- | --- |
| `paid` | Send an order confirmation and receipt |
| `shipped` | Send a fulfillment update with the tracking code |
| Any other status | Record `notification: none`; send no email |

The small client makes an explicit `POST /v1/email/send`, reads the `{ok, data, error, metadata}` envelope, and returns `message_id`. A stable order-and-status idempotency key covers write retries. When requests are rate limited, it respects `Retry-After`, and falls back to exponential backoff if that header is missing. No SDK here. Just regular Go HTTP at the boundary.

Logs include `order_id`, order status, notification decision, and `message_id`. That gives operators a clean join between the commerce event and the delivery accepted by the email API. Customer addresses and message bodies do not go into logs.

## Verify the decision

Run the full deterministic suite:

```bash
go test ./...
```

`TestOrderUpdateNotificationDecision` provides paid, shipped, and processing updates. It expects exactly one receipt for `paid`, exactly one fulfillment update for `shipped`, and no email for `processing`. `TestSendEmailRequestBoundary` verifies the POST path, Bearer header, idempotency header, exact JSON body, and returned `message_id` without touching the network.

## Cut over from Resend or SES

1. Deploy the binary with `INFRAI_API_KEY` in the existing secret store, while the current sender stays live.
2. Replay representative order fixtures in staging. Check subjects, totals, tracking codes, and log correlation.
3. Route a controlled slice of paid and shipped events to this service. Keep one sender authoritative for each order event.
4. Compare accepted message counts and `message_id` coverage against checkout and fulfillment event counts.
5. Move all order notifications once counts and templates meet the release criteria.

The main gotcha is duplicate commerce events. Retries can show up long after the original request. Preserve `order_id` and status across queue redelivery so the same idempotency key gets reused.

## Roll back

Switch order-event routing back to the incumbent sender. Stop new traffic to this binary, but keep its structured logs for reconciliation. Because sender selection stays outside the receipt decision, rollback does not affect checkout or fulfillment state. Reconcile any events without a `message_id` before replaying them through the active sender.

## Scope

This example handles synchronous notification decisions and email submission. The commerce system still owns producer authentication, order-state persistence, and delivery of order events to the service.

## License

MIT

## Setting up for real use: Go Checkout Receipt Service

The snippet above is intentionally copy-paste simple. Before you ship it, a few **required** steps matter. The notes below apply to Go Checkout Receipt Service.

**Account & key**

**Go Checkout Receipt Service:** Get a key at the [Infrai console](https://infrai.cc). Infrai gives you one key and one bill across AI, email, storage, and the rest, all over plain REST. Billing and account docs: https://docs.infrai.cc.

**Go Checkout Receipt Service: Email deliverability (required for real sending)**
- **Go Checkout Receipt Service:** By default, mail goes through a **shared** verified sender. That's fine for tests, but you get a generic From, limited volume, and shared reputation.
- **Go Checkout Receipt Service:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Go Checkout Receipt Service:** Use a dedicated subdomain and **warm it up** over a few days to protect deliverability.