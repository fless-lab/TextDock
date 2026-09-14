# TextDock

**Your local SMS development inbox.**

[Website & docs](https://fless-lab.github.io/TextDock/) · [Source](https://github.com/fless-lab/TextDock) · [Releases](https://github.com/fless-lab/TextDock/releases) · [Issues](https://github.com/fless-lab/TextDock/issues)

Capture messages, inspect SMS encoding, grab verification codes, and test your
application without a real SMS provider. One binary, one port, one local database.

TextDock is a Mailpit-style developer tool with phone pairing and delivery
simulation. Native SMS relays and an optional hosted team service are future
milestones. The local capture experience remains standalone and open source.

## Start here

With a release binary:

```sh
./textdock
```

Open **http://localhost:18257**. No configuration or account is required for
loopback use. Data is stored in `./data/textdock.db` relative to your working
directory. The default port `18257` avoids common development-server defaults;
it is configurable, not guaranteed to be unused or officially assigned.

From source (Go 1.26+, Node.js 24+, npm and Make):

```sh
make setup
make run
```

Node.js is only needed to build the UI. The compiled binary embeds all assets
and does not contact a CDN or cloud service.

Or build and run with Docker Compose:

```sh
docker compose up --build -d
```

Open the same URL and enter the local development token
`textdock-local-development`. The Compose port is loopback-only. Override the
host port with `TEXTDOCK_PORT=18259 docker compose up --build -d` and the token
with `TEXTDOCK_TOKEN`. The tag workflow publishes images to
`ghcr.io/fless-lab/textdock` after the first successful remote release.

## Send your first message

```sh
curl http://localhost:18257/api/v1/messages \
  -H 'Content-Type: application/json' \
  -d '{"to":"+33612345678","from":"Acme","body":"Your code is 482193","run_id":"signup-1"}'
```

If a server token is enabled (including Compose), add
`-H 'Authorization: Bearer YOUR_TEXTDOCK_TOKEN'`.

Wait for a code in a test, using a **new run ID for every execution**:

```sh
curl --get http://localhost:18257/api/v1/otp \
  --data-urlencode 'to=+33612345678' \
  --data-urlencode 'run_id=signup-1' \
  --data-urlencode 'timeout=30'
```

The OTP endpoint returns `{"code":"482193","message_id":"msg_…"}`. The
application remains responsible for generating, expiring and verifying codes.
Extraction is a heuristic for 4–8 digit candidates, not an authentication service.

## Available in v0.6.0-beta.3

This checkout includes the native-relay preview. The public documentation site
tracks the latest stable release. See [real relay setup and validation](docs/RELAY.md).

- Persistent SQLite inbox and responsive desktop/phone UI, light and dark themes.
- JSON capture API, search, recipient/run/time filters, per-message JSON export
  and deletion. Live updates via SSE and cursor-paginated message lists.
- GSM-7/UTF-16 analysis, estimated segments and detected OTP copy action.
- A bounded OTP wait endpoint for automated tests, isolated by recipient/run ID.
- A small, tested Twilio Messages create subset (`To`, `From`, `Body`).
- LAN access using a shared server token. This grants full inbox access.
- QR pairing with expiring one-use links, read-only recipient/run-scoped phone
  sessions, persistent mobile login and immediate revocation.
- Dedicated phone view, LAN address suggestions and configurable public origin.
- Projects/inboxes, favorites, tags, conversation filters and bulk actions.
- CLI workflows, full JSON/JSONL/CSV export, SQLite snapshots and opt-in retention.
- JSON configuration, custom OTP patterns and Unicode-trigger diagnostics.
- Seeded delivery scenarios, incoming SMS simulation and persistent status history.
- Signed webhooks with durable retries, attempt inspection and manual replay.
- A server-side SDK for local/Twilio/Vonage/OVH sending and documented provider capture subsets.
- Optional built-in Twilio relay and Android SIM gateway preview, with real-send labels and durable dispatch state.
- Idempotency, limits, queue expiration and native OTP message-format helpers.
- Binaries and Docker release workflow, automated API and browser checks.

**Not yet implemented:** push notifications, USB modem connectors, hardened multi-SIM device support and hosted
accounts. See the version-by-version [roadmap](docs/ROADMAP.md).

## Configuration

| Option | Environment | Default |
|---|---|---|
| `--listen` | `TEXTDOCK_LISTEN` | `127.0.0.1:18257` |
| `--db` | `TEXTDOCK_DB` | `data/textdock.db` |
| `--public-url` | `TEXTDOCK_PUBLIC_URL` | Optional phone-facing HTTP(S) origin |
| `--config` | `TEXTDOCK_CONFIG` | Optional JSON configuration file |
| `--retention` | `TEXTDOCK_RETENTION` | `0` (disabled) |
| `--otp-pattern` | `TEXTDOCK_OTP_PATTERN` | Default numeric heuristic |
| — | `TEXTDOCK_TOKEN` | Empty on loopback; 16+ characters required for network binding |
| `--version` | — | Print build version |

Flags override environment variables. Use `--db :memory:` for disposable test
environments. Change the port with `textdock --listen 127.0.0.1:18259`.
An occupied port causes startup to fail with the bind error; it never silently
changes the endpoint your application uses.

For the same-Wi-Fi phone interface:

```sh
TEXTDOCK_TOKEN='your-long-local-token' ./textdock --listen 0.0.0.0:18257
```

On the desktop, open **Open on phone**, select a recipient, then create and scan
the pairing QR code. The phone receives a separate read-only credential; the
desktop token is never included in the QR. Revoke devices from the same dialog.
For Docker/HTTPS, set `TEXTDOCK_PUBLIC_URL` to the phone-facing server origin.
This is a browser inbox, not delivery into the native SMS app. Use trusted LANs
for HTTP; remote access needs HTTPS. Details: [mobile and autofill](docs/MOBILE-AND-OTP.md).

## Documentation

- [Architecture and hosted evolution](docs/ARCHITECTURE.md)
- [UI design direction](docs/UI.md)
- [Projects, CLI, backups and test recipes](docs/DEVELOPER-WORKFLOW.md)
- [Delivery simulation and webhooks](docs/SIMULATION.md)
- [Provider compatibility and production switching](docs/PROVIDERS.md) · [Node SDK](packages/sdk/README.md)
- [Real SMS relay preview](docs/RELAY.md) · [Android companion](android/README.md)
- [Versioned roadmap and acceptance criteria](docs/ROADMAP.md)
- [Mobile, native SMS, WebOTP and Android/iOS](docs/MOBILE-AND-OTP.md)
- [Phone numbers and OTPs in tests](docs/TEST-NUMBERS.md)
- [Phone pairing and live connections](docs/CONNECT.md)
- [API and compatibility](docs/API.md) · [OpenAPI](api/openapi.yaml)
- [Release process](docs/RELEASING.md)
- [Website and Pages publishing](docs/WEBSITE.md)
- [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md)
- [French project overview](docs/PROJET.fr.md)

## License

MIT. TextDock is an independent project, not affiliated with Twilio or the mail
testing tools referenced above. Name availability has not been legally verified.
