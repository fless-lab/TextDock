# Scoped API keys

API keys let a CI job or integration work with one **project** or **inbox** without
receiving the TextDock operator token. Create and revoke them from **Manage
projects → Manage API keys**, or **Settings → Manage API keys**.

Machine keys are separate from [user accounts and project roles](USERS.md).
Audit trails and PostgreSQL remain follow-up milestones.

## Operator access and local mode

`TEXTDOCK_TOKEN` on the **server** remains the operator credential. It can manage
all projects, keys and device integrations. Set it when sharing an instance.

Zero-configuration loopback mode still permits anonymous operator access. Scoped
credentials are checked there too, but someone who can reach that anonymous API
can omit their credential. Creating a key does not enable server authentication.
Keep existing HTTPS/network setup when sharing the server with other machines.

## Create a key

Choose a name, project, optional inbox, permissions and expiration (up to 365 days).
The UI defaults to the current inbox, read/write permissions and 30 days. The
secret is displayed once; copy it into your CI secret store. It is not saved in
browser storage or returned in subsequent listings.

Example operator request, replacing project/inbox IDs and the expiration:

```sh
curl -H "Authorization: Bearer $TEXTDOCK_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Checkout CI","project_id":"default","inbox_id":"local","permissions":["messages:read","messages:write"],"expires_at":"2026-10-01T00:00:00Z"}' \
  http://localhost:18257/api/v1/keys
```

The response contains `key` metadata and a `td_key_…` token. `GET /api/v1/keys`
returns metadata only, including revoked keys. `DELETE /api/v1/keys/{id}` revokes
a key idempotently. To rotate or change scope/permissions, create a replacement,
update the integration and revoke the old key.

## Permissions

| Permission | Allowed operations |
|---|---|
| `messages:read` | List messages, read OTPs, export JSON/CSV, inspect message events, subscribe to scoped invalidations |
| `messages:write` | Capture local SMS; edit tags/favorites when `messages:read` is also present |
| `messages:delete` | Delete individual/batched messages and purge an allowed inbox |

Every valid key can inspect its own metadata at `GET /api/v1/session`, basic server
information and the project/inbox list filtered to its scope. Write does not imply
read or delete; a capture response contains the message just submitted. Metadata
editing needs both read and write because it returns the updated message.

For this preview, **simulation, inbound simulation, custom callbacks, real relay,
gateway/lab operations, phone pairing, device management, push preferences and key
administration require their existing operator/device credentials**. Unsupported
and future operator routes are denied by default for API keys.

## Scope selection

- An **inbox key** defaults omitted inbox selectors to its own inbox. Explicitly
  asking for another inbox fails with 403.
- A **project key** covers current and future inboxes in that project. Requests
  operating on an inbox must select one explicitly. The API does not silently
  fall back to the global local inbox.
- An unknown or out-of-scope message ID returns 404. Batched deletion validates
  all IDs before deleting anything; a missing or out-of-scope ID rejects the batch.
- Export, pagination and OTP waits preserve the inbox constraint. A `run_id` is a
  test filter, not an authorization boundary.

## Use with the API, CLI and SDK

Send the API key as a Bearer credential:

```sh
curl -H "Authorization: Bearer $TEXTDOCK_API_KEY" \
  'http://localhost:18257/api/v1/messages?inbox=local'
```

For a CLI/SDK **client**, `TEXTDOCK_TOKEN` can hold the restricted key:

```sh
TEXTDOCK_TOKEN="$TEXTDOCK_API_KEY" textdock list --inbox local
```

Use the actual allowed inbox ID. The existing CLI and SDK default to `local`, so
configure an inbox explicitly when your key belongs elsewhere:

```js
import { createSMS } from '@textdock/sdk';
const sms = createSMS({
  driver: 'local',
  baseURL: 'http://localhost:18257',
  token: process.env.TEXTDOCK_API_KEY,
  inbox: 'inbox_your_test_inbox',
});
await sms.send({ to: '+12025550123', from: 'Checkout', body: 'Code 482193', runId: 'ci-42' });
```

The local provider-shaped capture endpoints also accept scoped keys: Twilio Basic
password, Vonage `api_secret`, OVH `X-Ovh-Consumer`, or Bearer authorization.
Select the inbox with `X-TextDock-Inbox`. These are credentials for **TextDock's
local adapters**, not credentials accepted by the actual SMS providers.

Basic authorization is supported only on the Twilio adapter. API keys are not
accepted through URL query parameters and cannot be used as phone or gateway
credentials. The desktop application is the operator management UI; this preview's
scoped-key clients are the API, CLI and SDK.

## Storage, expiration and revocation

SQLite schema 9 stores SHA-256 hashes of 256-bit random secrets, plus scope,
permissions, name and lifetime. Tokens are never recoverable through the key API.
Backups include the hashes and live grants; restoring a backup can restore a key's
earlier revocation state, so review keys after a restore.

Each request authenticates against current database state. OTP polling rechecks
the credential, and revocation closes an open SSE stream with a `revoked` event.
Streams are bound to one allowed inbox and expire with the key. Requests already
admitted may finish; revocation does not undo completed captures/deletions.

See [the API contract](../api/keys.openapi.yaml) and [security notes](../SECURITY.md).
