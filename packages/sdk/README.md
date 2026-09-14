# @textdock/sdk

A small, dependency-free **Node.js 20.3+ ESM** SMS client. TypeScript declarations
are included. One send interface, with explicit `local`, `twilio`, `vonage`, or
`ovh` drivers. Provider credentials stay in your backend process.

Install from the TextDock GitHub Release asset:

```sh
npm install https://github.com/fless-lab/TextDock/releases/download/v0.7.0-beta.1/textdock-sdk-0.7.0-beta.1.tgz
```

Or install `./packages/sdk` from a source checkout. This package is distributed
as a release tarball; no npm-registry publication is assumed.

```js
import { fromEnv } from '@textdock/sdk';

const sms = fromEnv();
const result = await sms.send({
  to: '+33612345678',
  from: 'Acme',
  body: 'Your code is 482193',
  runId: 'signup-42',
});
console.log(result.id, result.status, result.provider);
```

## Configure a driver

| Driver | Environment |
|---|---|
| Local (default) | `SMS_DRIVER=local`, optional `TEXTDOCK_URL`, `TEXTDOCK_TOKEN`, `TEXTDOCK_INBOX`, `TEXTDOCK_SCENARIO` |
| Twilio | `SMS_DRIVER=twilio`, `TWILIO_ACCOUNT_SID`, `TWILIO_AUTH_TOKEN` |
| Vonage SMS API | `SMS_DRIVER=vonage`, `VONAGE_API_KEY`, `VONAGE_API_SECRET` |
| OVH SMS jobs | `SMS_DRIVER=ovh`, `OVH_APP_KEY`, `OVH_APP_SECRET`, `OVH_CONSUMER_KEY`, `OVH_SMS_SERVICE` |

`SMS_ENDPOINT` overrides the selected driver endpoint. OVH's default is the EU
`https://eu.api.ovh.com/1.0` API; use the appropriate regional endpoint for your
account. Credentials and authorized sender/recipient settings are required for
real providers. Non-local drivers make actual provider requests.

Or configure directly:

```js
import { createSMS } from '@textdock/sdk';
const sms = createSMS({ driver: 'local', baseURL: 'http://localhost:18257', inbox: 'local' });
```

`send` takes `to`, `from`, `body`, plus optional `runId`, `scenarioId`, `callbackURL`
and `signal`. `runId` and `scenarioId` are TextDock-only metadata. Local callbacks
require a selected simulation scenario. Twilio/Vonage accept a per-message
callback URL; OVH requires service-level callback configuration and rejects this
option. Only plain text, single-recipient sending is covered.

For the local driver, `mode: 'relay'` (or `TEXTDOCK_MODE=relay`) requests an
explicitly enabled real relay on the server. Capture is still the default. Pass
`idempotencyKey` to reuse one send intent across retries. A relay-mode call without
a supplied key generates one for that call; the SDK does not retry it itself.
An interrupted carrier send remains uncertain and requires reconciliation.

Vonage chooses GSM-compatible `text` or `unicode` based on message characters.
OVH requests server time before signing the exact POST body with AK/AS/CK
credentials. OAuth, media, templates, bulk sends, verification products and
provider scheduling are outside this client subset.

## Errors and delivery semantics

`SMSError` exposes `provider`, optional HTTP `status`, provider `code`,
`retryAfter`, and `uncertain`. A network failure after starting a POST, malformed
success response or server error may leave acceptance uncertain. The SDK does
**not** automatically retry sends; reconcile provider state before retrying an
uncertain request. This avoids turning a transport timeout into duplicate SMS.

Default request timeout is 10 seconds; configure `timeout` in milliseconds
(1–60000) and/or provide an `AbortSignal`. Responses are limited to 1 MiB and
redirects are not followed. Acceptance is not delivery: use real provider
receipts to confirm arrival in production.

Protocol/normalization tests run against local receivers and TextDock's capture
adapters. No live carrier delivery is implied. See the repository's provider
support matrix for the precise emulation subset.
