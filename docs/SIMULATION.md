# Delivery simulation and webhooks

## Three distinct operations

- **Capture** records a message, with no lifecycle or external callback.
- **Simulate delivery** records an outbound message and advances its local
  lifecycle through `queued → sent → delivered/failed/expired`.
- **Simulate incoming SMS** records `received` and can POST an inbound callback
  to your application, for flows such as replies, `STOP` and `HELP`.

None sends a real SMS. Configured webhooks are real HTTP requests to your test
application. The application implements its own opt-out/reply behavior; TextDock
does not register carrier-side subscriptions or emulate an entire operator.

## Create and select a scenario

Open **Scenarios** in the current inbox, then create a named rule with:

- optional recipient prefix;
- terminal outcome and delay (0–86,400,000 ms);
- failure percentage and seed;
- optional HTTP rejection (400, 429 or 503), with `Retry-After`;
- optional callback URL and JSON or Twilio form format.

Choose **Simulate delivery** in the composer and select the scenario. API:

```json
{
  "inbox": "local",
  "to": "+33612345678",
  "from": "Acme",
  "body": "Your code is 482193",
  "run_id": "retry-test-42",
  "scenario_id": "scenario_REPLACE_ME"
}
```

A scenario ID enables simulation. Otherwise `mode` defaults to `capture`.
Scenario inbox and prefix must match. HTTP rejection happens before a message
is persisted. A rejection rule returns its configured error on every matching
request; this is a deterministic fault fixture, not a sliding-window quota meter.

Failure selection hashes the seed, recipient, run ID and body. Repeating those
values gives the same outcome even though message IDs/timestamps differ. The
delay is scheduling intent, not a delivery-time guarantee under load. The
worker exposes a clock-driven step for tests; there is no UI time-travel control.

CLI:

```sh
textdock send --to +33612345678 --body 'Code 482193' --scenario scenario_REPLACE_ME --run-id retry-test-42
textdock wait --to +33612345678 --run-id retry-test-42 --status delivered
textdock send --direction inbound --to +33612345678 --from +33699999999 --body STOP --callback-url http://localhost:3000/inbound
```

OTP retrieval otherwise includes any matching status. Use `status=delivered`
when a test must wait for simulated delivery, not just capture.

## Twilio capture SDK

The create endpoint accepts `StatusCallback` when `X-TextDock-Scenario` selects a
simulation scenario. Use `X-TextDock-Inbox` for a non-default inbox. SDK header
overrides depend on the language/client; the plain JSON API is the easiest path
when your SDK doesn't expose custom transport headers.

Simulated Twilio receipts contain `MessageSid`, `SmsSid`, `MessageStatus`,
`SmsStatus`, `To`, `From`. They implement this subset rather than the complete
Twilio webhook schema. Message SID agrees with the create response.

## Signing

Set `TEXTDOCK_WEBHOOK_SECRET` before starting the server:

- TextDock JSON: `X-TextDock-Signature: sha256=<hex HMAC-SHA256 of raw body>`.
- Twilio form: `X-Twilio-Signature`, using Twilio's URL + sorted form values and
  HMAC-SHA1/base64 algorithm. Tests verify it using the official Twilio Node SDK.

Without a configured secret, callbacks are unsigned. The signing key is not
stored in SQLite or returned through the API. Request headers and bodies are
stored in the local attempt log, including the resulting signature.

JSON callbacks contain `event_id`, `type` (e.g. `sms.delivered`) and a message
snapshot. `X-TextDock-Event-ID` is the stable delivery-job key for retry/replay
deduplication. Consumers should accept duplicate deliveries safely.

## Durability, ordering and retries

SQLite transactions save the message, first event and scheduled jobs together.
Transitions update status, append an event and enqueue callbacks in one commit.
Workers lease jobs for 30 seconds, recover expired leases after restart, and fence
stale workers using a monotonic attempt number. Later transitions wait for earlier
ones, so a crash between claim and commit cannot turn `delivered` back into `sent`.

Callbacks execute outside DB transactions with a five-second timeout. Only 2xx
is success. Redirects are recorded, not followed. Up to five attempts use
exponential retry delays. **Events** shows request headers/body, response preview
(4096 bytes), HTTP status, network error and duration. **Replay callback** schedules
another attempt budget after completion/failure. Replaying an older receipt lets
you test duplicate or out-of-order callbacks.

This is at-least-once HTTP delivery: if a process dies after the receiver accepts
a request but before the attempt is committed, the job may be delivered again.
Deleting/purging/expiring a message removes its queued jobs and callback logs by
foreign-key cascade; a request already in flight may have reached its receiver.
Deleting a scenario does not change jobs already scheduled from its snapshot.

## Inbound API

`POST /api/v1/inbound` accepts the normal message fields with an optional
`callback_url`. It forces inbound simulation and uses JSON callbacks. Example:

```json
{
  "inbox": "local",
  "to": "+33612345678",
  "from": "+33699999999",
  "body": "STOP",
  "callback_url": "http://localhost:3000/inbound"
}
```

Scenarios, histories, attempts and replay endpoints require desktop access.
Paired devices can read only scoped messages and invalidation events.
