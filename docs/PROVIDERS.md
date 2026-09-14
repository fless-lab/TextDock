# Provider integration and production switching

v0.5 introduces a server-side SDK with one `send` interface and four drivers:
TextDock, Twilio, Vonage SMS API and OVH SMS jobs. Its runtime has no external
dependencies. See [SDK usage](../packages/sdk/README.md).

```js
import { fromEnv } from '@textdock/sdk';
const sms = fromEnv();
await sms.send({ to: '+33612345678', from: 'Acme', body: 'Your code is 482193' });
```

Development uses `SMS_DRIVER=local`; production selects a real provider and its
credentials. The SDK is intended for application backends, not browser bundles.
It makes real provider requests for non-local drivers. Accepted/queued means a
request was accepted; provider receipts confirm actual delivery.

## Exact capture-adapter matrix

These endpoints emulate **declared request/response subsets**, not whole vendor
platforms. They always capture/simulate locally, even when using a provider-shaped
route. No provider credentials are needed to use them.

| Adapter | Endpoint | Supported input | Success shape |
|---|---|---|---|
| Twilio | `POST /2010-04-01/Accounts/{sid}/Messages.json` | Form `To`, `From`, `Body`; `StatusCallback` with explicit scenario | SID, body, addresses, queued status and segment count |
| Vonage | `POST /sms/json` | JSON/form `api_key`, `api_secret`, `to`, `from`, `text`, optional text/unicode `type` | `message-count`, one message with status `0` and ID |
| OVH | `GET /1.0/auth/time` | No body | Unix timestamp |
| OVH | `POST /1.0/sms/{service}/jobs` | One receiver, sender, message, optional high priority and `noStopClause: false` | Numeric job ID, receiver lists, zero capture credits |

All adapters normalize into the same message/inbox model. Optional headers
`X-TextDock-Inbox`, `X-TextDock-Run-ID` and `X-TextDock-Scenario` set development
context. Forced Vonage Unicode encoding is reflected in SMS analysis.

Unsupported fields fail instead of being silently forwarded. Media, templates,
bulk recipients, provider scheduling, OAuth flows, Verify products, real balances,
number ownership and carrier rewriting are not emulated. Vonage callback fields
and OVH opt-out-clause behavior are outside the capture subset. Provider-shaped
error payloads are not fully emulated; TextDock HTTP/JSON errors are authoritative.

For token-protected TextDock instances:

- Twilio Basic-auth password is the TextDock token.
- Vonage `api_secret` is the TextDock token.
- OVH `X-Ovh-Consumer` is the TextDock token.
- A Bearer TextDock token can also protect the Vonage/OVH capture routes.
- Device-session credentials never authorize sending.

OVH capture accepts SDK signature headers but does not validate real AK/AS/CK
credentials. It does not implement credential issuance/OAuth. The *production SDK*
signs requests using configured OVH credentials and server time. Choose the
correct regional endpoint for your account.

## Verification scope

- Official Twilio Node SDK create and webhook signature contracts are exercised.
- The TextDock SDK's four drivers are tested end-to-end against local capture
  adapters, with additional independent HTTP fixtures for body/auth mapping,
  OVH request signing and Vonage errors inside HTTP 200.
- The SDK recognizes uncertain transport outcomes and never retries sends
  automatically. Delivery receipts, deduplication and retry policy remain explicit.
- Live credentialed sends, sender approval, country restrictions, opt-out text
  rewriting, fees and physical-phone delivery require provider integration tests.
  The automated suite contains no real provider credentials and sends no carrier SMS.

## References

- [Twilio Message resource](https://www.twilio.com/docs/messaging/api/message-resource)
- [Vonage SMS API](https://developer.vonage.com/en/api/sms)
- [OVH Node client and SMS authorization example](https://github.com/ovh/node-ovh)
- [OVH EU API](https://eu.api.ovh.com/console/)
