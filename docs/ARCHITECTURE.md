# Architecture

## Product constraints

1. A downloaded binary starts a useful local inbox with no account or sidecar.
2. UI and API share one configurable port, default **18257**.
3. Captured, simulated and actually delivered are distinct concepts.
4. Provider compatibility is declared per operation, never implied wholesale.
5. Mobile capabilities obey platform boundaries; a web inbox is not an SMS.
6. A hosted offering must not make local development depend on the service.

## Implemented layout

```text
cmd/textdock/             Configuration, listener, lifecycle, dependency wiring
internal/message/        Canonical model, validation, encoding and OTP analysis
internal/storage/        SQLite adapters, transactional migrations and snapshots
internal/workspace/      Project/inbox persistence contract
internal/cli/            Scriptable API clients and backup/restore commands
internal/config/         Strict JSON runtime configuration
internal/connect/        Scoped device credentials and pairing repository contract
internal/events/         Scoped/coalesced invalidation hub
internal/application/    Provider-neutral capture/simulation orchestration
internal/simulation/     Scenarios, callback formatting and durable worker
internal/httpapi/        HTTP boundary, JSON API, Twilio create subset, token auth
internal/ui/             Embedded production UI assets
web/src/                 Desktop UI, PhoneApp, components and live-event hook
web/tests/               Real browser and official provider SDK contract tests
api/openapi.yaml         Public v1 JSON API contract
docs/                    Decisions, roadmap, platform constraints, release process
scripts/                 Release metadata validation
packages/sdk/            Dependency-free Node sender drivers and TypeScript contract
.github/workflows/       CI and tag-driven binary/container releases
```

Go's standard HTTP server, React, Vite and pure-Go SQLite form a **modular
monolith**. SQLite is deliberately the only runtime persistence dependency.
No Redis, message broker, Node server or third-party font is needed to run it.

`message.Repository` is the implemented storage boundary. The domain package
imports neither HTTP nor SQLite. HTTP validates transport shape and maps inputs
into the domain. Startup owns storage lifetime and graceful shutdown.

The API source marker records `api` or `twilio`; it never suggests real network
delivery. The stored state is `captured`. Twilio's `queued` is a compatibility
response only. The database contains message payloads and lookup columns,
parameterized queries, WAL journaling, and one connection for predictable local
behavior. A schema-version table records the initial schema. Future migrations
must be sequential and transactional; merely creating new tables is not a
migration strategy for subsequent releases.

The frontend has two entry views: desktop `/` and read-only `/phone`. Browser
state handles theme, selection and separate desktop/device credentials. A
fetch-based SSE hook reconnects with bearer headers and invalidates scoped
queries; every new connection starts with a full resync. Requests are cancelled
on scope/search changes, preventing stale search responses.
The API returns at most 200 rows and the UI renders 100 per cursor page.
v0.3 adds project/inbox scope, metadata filters, consistent SQLite snapshots and
optional retention. Every connection re-applies foreign-key enforcement; a
separate keeper connection preserves memory databases during cancellation-driven
request-connection recycling.

## Planned modules, added with their first working feature

These are architecture decisions, **not empty packages pretending to work**.

| Boundary | Responsibility | First version |
|---|---|---|
| `connect` | One-use pair codes, hashed device credentials, scopes, revocation | Implemented v0.2 |
| `events` | Append-only lifecycle timeline, SSE notifications with resync | v0.2–v0.4 |
| `workspace` | Local projects, inboxes, test runs, scoped access | Implemented v0.3 |
| `simulation` | Seeded scenarios, injected worker clock, rejection/latency rules | Implemented v0.4 |
| Callback worker | Durable outbox, signing adapters, attempts and replay | Implemented v0.4 |
| Provider adapters / SDK | Declared request subsets, sender drivers and error normalization | v0.5 subsets |
| `relay` | Explicit real-send routing, connector jobs and delivery receipts | v0.6 |
| `devices` | Android gateway enrolment and emulator transport | v0.6–v0.7 |
| `cloud` | Organizations, identities, tenant-aware services and quotas | v0.9 |

v0.4 extracts capture orchestration into an application service. Transactions
save messages, initial events and jobs together. A leased worker applies ordered
transitions and schedules callbacks transactionally. HTTP callbacks execute
outside transactions with at-least-once semantics and fencing on attempt commit.

### Three execution modes

- **Capture**: persist and inspect; no external delivery.
- **Simulate**: deterministic transitions and callbacks; no external delivery.
- **Relay**: explicitly dispatch through a selected connector; record attempted,
  accepted, delivered, failed or uncertain separately.

A future `Sender` boundary takes a canonical outbound request and idempotency
key, returning a provider reference and acceptance state. Capability metadata
describes supported sender types, encoding, callback signatures and inbound
messages. Unsupported options must fail visibly instead of being dropped.

Retries are safe only when a connector can deduplicate or reconcile an uncertain
submission. A timeout is not proof that a real SMS was never sent. Gateway jobs
need leases, acknowledgements and a durable attempt log before automatic retries.

### Mobile pairing

The desktop creates an expiring, one-use challenge. QR content uses a URL
fragment, exchanged for a separate device session after opening the page. The
server stores only credential hashes, binds sessions to an inbox/recipient scope
and permits revocation. A paired device receives read-only access by default,
not the desktop's admin token. This flow is implemented in v0.2. Code lifetime,
replay, concurrent claims, stream revocation, restart persistence and scope
enforcement have automated tests. v0.1's manually entered shared token is
explicitly not this mechanism.

### Notifications and connectivity

Same-LAN browser access is the zero-service baseline. Automatic LAN address
suggestions and QR pairing reduce setup in v0.2. HTTPS/tunnels are optional and
explicit; do not silently upload inbox contents. PWA installation/Web Push need
secure contexts and platform support, and push usually uses external platform
services. Notification payloads should omit OTP contents by default.

## Local to hosted

Build a hosted control plane around the same capture engine rather than
retrofitting tenant isolation into unscoped SQL queries after launch.

1. Introduce `Scope { organization, project, inbox }` as a mandatory application
   boundary. In local mode it resolves automatically to one default workspace.
2. Make every read, write, event subscription, pairing, export and webhook job
   carry a scope. Prove cross-tenant denial in integration tests.
3. Add PostgreSQL storage/migrations and independently deployable workers using
   the transactional outbox. SQLite remains the local adapter.
4. Add OIDC/session identities, project API keys, roles, audit logs, retention,
   quotas and billing metering to the hosted control plane.
5. Add optional authenticated local-to-cloud sharing with visible boundaries,
   never an implicit mirror of all development messages.

`run_id` is a **test filter, not a tenant authorization boundary**. The shared
v0.1 server token grants complete local access. This release is not a hosted
multi-tenant service.

The intended business model is an open-source local/self-hosted core plus paid
managed hosting, collaboration, storage and support. Hosting operations can be
separate; license changes or closed premium components require an explicit
product decision. MIT is the current repository license.

## UX and performance budget

- First message in under a minute with a downloaded binary.
- One port, one process, one data directory. No runtime package installation.
- Usable phone layout, keyboard focus, native modal focus containment, visible
  failure/reconnection states, and no fake or inactive navigation destinations.
- Follow the [sparse, utility-first visual direction](UI.md): factual text and
  message content take priority over decoration.
- Target: initial JS+CSS below 150 KiB gzip; track bundle output before releases.
- Targets to measure before v1: idle RSS below 60 MiB, startup below one second,
  responsive search at 100k messages. These are goals, not verified claims.
- Add virtualization, indexes and cursor pagination before unlimited UI loading.
