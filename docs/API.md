# API v1

Base URL: `http://localhost:18257`. JSON requests/responses use UTF-8. If configured,
all `/api/` endpoints require `Authorization: Bearer <TEXTDOCK_TOKEN>`.
`/healthz` is a public process liveness probe. UI assets are public; message APIs
remain protected when authentication is enabled. CORS is not enabled: call from
your backend/test runner or the same-origin UI.

v0.2 adds desktop pairing/device administration and separate scoped phone APIs.
See [CONNECT.md](CONNECT.md) and [connection OpenAPI](../api/connect.openapi.yaml).
The phone bearer credential does not authorize any `/api/` request.

## JSON API

| Method | Route | Result |
|---|---|---|
| GET | `/healthz` | Liveness, 200 |
| GET | `/api/v1/info` | Version/mode/auth configuration |
| POST | `/api/v1/messages` | Capture, 201 with message |
| GET | `/api/v1/messages` | `{messages: [], limit: 100}` |
| DELETE | `/api/v1/messages/{id}` | 204, or 404 if absent |
| GET | `/api/v1/otp` | `{code, message_id}`, or 404 on timeout |

v0.4 adds simulation scenarios, lifecycle events, inbound messages and callback
attempts/replay. See [SIMULATION.md](SIMULATION.md) and the
[simulation contract](../api/simulation.openapi.yaml).

Create fields: `to` (E.164-shaped `+` and 7–15 digits, no real carrier validation),
`from` (required, up to 64 characters), `body` (nonblank, up to 4096 Unicode
characters), optional `run_id` (up to 128 bytes). Unknown JSON fields and multiple
JSON documents are rejected. HTTP bodies are capped at 32 KiB.

List filters:

- `q`: case-insensitive SQLite text search over body, or literal recipient
  substring. SQLite's default case folding is primarily ASCII; no promise of
  full Unicode linguistic search. `%`/`_` are literal, not SQL wildcards.
- `to`: exact recipient; URL-encode the plus sign (`%2B`).
- `run_id`: exact test run identifier.
- `since`: inclusive RFC3339 timestamp.
- `limit`: 1–200, default 100; newest first.
- `inbox`: inbox ID, defaults to `local`; paired devices enforce their stored inbox.
- `cursor`: opaque `next_cursor` from the preceding page, keeping filters unchanged.
- `favorite`, `otp`: booleans to select favorites or detected-code messages.
- `tag`: exact metadata tag.
- `status`: exact lifecycle status, also usable when waiting for an OTP.

v0.3 responses include `next_cursor`, empty on the final page.
Project management and export routes are in [workspace OpenAPI](../api/workspace.openapi.yaml).

OTP uses the same filters plus `timeout` (0–30 whole seconds, default 0), and
**requires** both `to` and `run_id`. It returns the first candidate from newest
matching messages in the bounded result set. Every execution should have a new
run ID; reuse can return an earlier code. This is not a queue or one-time consume
endpoint. Default OTP extraction recognizes 4–8 digit standalone candidates.
`--otp-pattern` can replace it with the first capture group of a custom RE2
pattern (up to 64 bytes); otherwise read the complete body for other formats.

Errors are `{ "error": "description" }`: 400 malformed/invalid request, 401 token,
403 browser-origin/Host boundary, 404 missing resource/OTP, 415 wrong Twilio form
media type, 422 unsupported Twilio option, 500 internal failure. Method errors
may be 404 at the versioned API dispatcher. See [OpenAPI](../api/openapi.yaml).

## Twilio compatibility matrix

This is an emulation subset, not a replacement for all Twilio services.

| Operation / behavior | v0.1 |
|---|---|
| `POST /2010-04-01/Accounts/{AccountSid}/Messages.json` | Supported |
| URL-encoded `To`, `From`, `Body` | Supported |
| Response SID, body, addresses, `queued`, segment count | Supported |
| Official Twilio Node SDK `messages.create` | Contract-tested |
| Basic auth | Password is TextDock token, when enabled |
| Real account/number ownership validation | No |
| `StatusCallback` with `X-TextDock-Scenario` | Simulated receipts, optional Twilio signature |
| Media/MMS, Messaging Service SID, provider scheduling | Rejected with 422 |
| Message fetch/list/delete in Twilio format | Not implemented |
| Twilio Verify | Not implemented |
| Delivery simulation | v0.4 scenarios; declared receipt subset only |
| Full Twilio error codes/response schema | Not implemented; TextDock errors |

Default stored messages have `status: captured`; `queued` is the SDK-facing
acceptance representation. Explicit scenarios store `queued` and run the local
simulation worker. No mobile network delivery occurs.

Example in a Node application that has `twilio` installed:

```js
import twilio from 'twilio';

const client = twilio('AC' + '0'.repeat(32), process.env.TEXTDOCK_TOKEN || 'local');
client.api.baseUrl = 'http://localhost:18257';

const result = await client.messages.create({
  to: '+33612345678',
  from: 'Acme',
  body: 'Your code is 482193',
});
console.log(result.sid);
```

Production uses actual credentials and the default Twilio endpoint. Carrier
constraints, authorized senders and real callbacks still require integration
testing. Other language SDKs may require different endpoint override mechanisms;
only the Node SDK is verified in this release.
