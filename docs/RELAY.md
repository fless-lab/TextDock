# Real SMS relay — v0.6 preview

Relay is **disabled by default**. Normal capture and simulation remain local.
When explicitly enabled, `mode: relay` queues a real SMS through Twilio or an
enrolled Android SIM gateway. The composer labels the action **Send real SMS**.

This is a prerelease: automated protocol tests and the Android APK build pass,
but a physical SMS/autofill test is still required before declaring v0.6 stable.

## Twilio

```sh
TEXTDOCK_RELAY_DRIVER=twilio \
TWILIO_ACCOUNT_SID=YOUR_ACCOUNT_SID \
TWILIO_AUTH_TOKEN=YOUR_AUTH_TOKEN \
textdock
```

Use an authorized sender number or sender ID and a real recipient you control.
The built-in transport is Go code; Node.js is not needed to run the relay.
Credentials stay in server environment variables, not SQLite or the UI.

To receive delivery callbacks, also configure `TEXTDOCK_PUBLIC_URL` as the
public HTTPS origin of this server, together with `TEXTDOCK_TOKEN`. Each request
includes a callback URL correlated to its TextDock message ID. The endpoint
verifies Twilio signatures and binds the provider SID before updating state.
Late receipts cannot regress a delivered message to sent/accepted.

Without a reachable callback origin, the relay records provider acceptance but
cannot confirm destination delivery. `TEXTDOCK_RELAY_ENDPOINT` overrides the
Twilio origin for protocol tests; it is normally left unset.

## Android SIM gateway

```sh
TEXTDOCK_RELAY_DRIVER=android \
TEXTDOCK_TOKEN='your-long-local-token' \
textdock --listen 0.0.0.0:18257
```

Create a gateway token in **Relay**, install the development APK, and enter the
server origin/token in the app. Start the gateway and grant SMS permission.
The first available enrolled gateway claims queued Android jobs. Use a separate
destination phone for receipt/autofill testing; self-sending is not assumed.

Gateway credentials are distinct from phone-inbox credentials. They can claim
outbound jobs and acknowledge their own leases, but cannot read the desktop API,
send arbitrary new jobs or acknowledge another gateway's job. Tokens expire
after 30 days and can be revoked. Only hashes are stored server-side.

See the [Android companion instructions](../android/README.md).

## Queue, limits and uncertain outcomes

| Setting | Default | Meaning |
|---|---|---|
| `TEXTDOCK_RELAY_DRIVER` | Empty | `twilio` or `android`; empty disables relay |
| `TEXTDOCK_RELAY_LIMIT` | `10` | Logical messages/minute, enforced on admission and dispatch |
| `TEXTDOCK_RELAY_TTL` | `10m` | Maximum queued time before dispatch; 1s–24h |
| `TEXTDOCK_RELAY_ALLOWED_TO` | Empty | Optional comma-separated exact recipient allowlist |

Limits count logical messages, not billed SMS segments. A queue TTL expiring
before dispatch produces `expired`. A dispatch lease interrupted after claiming
produces `unknown`; it is never put back into the queue automatically.

Twilio 2xx responses with a SID produce `accepted`, signed callbacks can advance
to `sent`/`delivered`/`failed`, and 4xx rejections produce `failed`. Network errors,
5xx responses and unreadable success responses produce `unknown`. Inspect the
provider or gateway before issuing a new intent after an uncertain outcome.

Android reports `sent` after its successful submission callbacks, not recipient
delivery. An expired/revoked gateway or interrupted app can leave an uncertain
job. The server keeps its state rather than guessing whether to resend.

Deleting/purging a queued message cancels its queued job. A request already in
flight may have reached the carrier; removing a database row cannot undo that.
Disabling the relay stops new dispatch; queued jobs still expire by their TTL.
Keep the Twilio signing credential and public origin configured if late receipts
for previously submitted messages should still be accepted after dispatch is disabled.

## Idempotency

```json
{
  "mode": "relay",
  "inbox": "local",
  "to": "+YOUR_REAL_NUMBER",
  "from": "YOUR_AUTHORIZED_SENDER",
  "body": "Your code is 482193",
  "idempotency_key": "signup-send-42"
}
```

Use one stable key per send intent. The same inbox/key and SMS content return
the original message without sending again. Different content or a deleted
original produces 409. Key fingerprints survive message deletion so an old key
cannot recreate a carrier send. Keys are optional on the raw API; callers that
omit them create a new intent on every POST.

The UI retains a key for a composer submission. The Node SDK generates a key for
one `send` call in relay mode; pass `idempotencyKey` yourself if retrying across
calls. The SDK still does not automatically retry sends.

```sh
textdock send --mode relay --to +YOUR_REAL_NUMBER --from YOUR_SENDER \
  --body 'Your code is 482193' --idempotency-key signup-send-42
```

## Native OTP formatting

The Relay dialog includes WebOTP and Android SMS Retriever format helpers.
`POST /api/v1/otp/format` formats an application-generated code:

```json
{"format":"webotp","code":"482193","domain":"login.example.test"}
```

Or provide `format: android` and the correct 11-character `app_hash`. This checks
syntax and builds the SMS text; it does not generate a code, provision a number,
verify a signing certificate or trigger native autofill by itself.

## Physical validation before stable v0.6

Record the phone models, OS/browser versions, provider/SIM and application origin:

1. Send to an owned physical test phone and confirm receipt in its native inbox.
2. Test WebOTP on the correct trusted HTTPS origin, including consent/fallback.
3. Test Android Retriever with the app's actual signing-specific hash.
4. Test iOS `.oneTimeCode` suggestions using a real received SMS.
5. Disconnect/restart the gateway during submission and reconcile unknown jobs
   without duplicate sends; exercise SIM errors, multipart text and permission denial.

These checks cannot be replaced by a browser filling a field or a fake gateway
acknowledging a job in an automated test.
