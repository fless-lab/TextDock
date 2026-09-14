# Versioned roadmap

Each minor version is one useful, tested user journey. Make focused commits
during development, then one annotated tag only after the acceptance criteria
and release checks pass. Planned features below are not implemented by scaffolding.

## v0.1.0 — Local capture foundation (released)

Scope: embedded responsive inbox, SQLite, JSON capture/search/delete, encoding
inspection, OTP extraction/wait API, explicit Twilio create subset, LAN shared
token access, source/Docker builds, CI and release automation.

Acceptance: start one binary, send from curl and the official Twilio Node SDK,
inspect the OTP on desktop/mobile browser, isolate a test run, restart without
losing messages, reject unsupported provider options. Automated tests cover these
core paths; physical-device and hosted release checks are recorded separately.

## v0.2.0 — Connect your phone (released)

- LAN address discovery, copyable connection URL and QR pairing.
- Single-use/expiring pairing challenges, scoped read-only device sessions,
  device list and revoke action.
- Dedicated phone conversation view and per-recipient subscriptions.
- SSE live updates, reconnect indication and full resync after missed events.
- Clipboard fallback, local HTTPS guide, PWA manifest and install guidance.

Acceptance: scan → choose inbox → see a new message without manual refresh.
Expired/replayed pair codes fail. A revoked phone cannot read/list/subscribe.
The mobile session cannot delete messages or read another scope.

## v0.3.0 — Daily developer workflow (released)

- Projects/inboxes, test-run views, conversations, tags and favorites.
- Cursor pagination, indexed advanced search, bulk actions and JSON/CSV export.
- Configurable retention, data backup/restore and transactional migrations.
- Config file and CLI send/list/wait/purge commands.
- Dependency-free Node test helper and Playwright/Cypress/backend recipes.
- Improved OTP candidates with explicit patterns; GSM character highlighting.
- Command palette, keyboard shortcuts, automated accessibility checks and UI regression set.

Acceptance: repeated parallel tests never pick an earlier run's OTP; browse a
100k-message fixture without unbounded responses or rendering all rows at once.

## v0.4.0 — Simulate the difficult cases (implemented)

- Durable capture/queue acceptance timeline: queued, sent, delivered, failed, expired.
- Seeded scenarios by recipient/prefix, delays, invalid numbers, rate limits,
  provider outages and retry-after responses.
- Signed provider-style delivery callbacks and simulated inbound SMS.
- Durable outbox, retries, request/response inspection and manual replay.
- STOP/HELP test scenarios for application behavior.

Acceptance: a saved scenario reproduces the same event sequence; crash/restart
does not lose scheduled events or webhook attempts. Timeouts and out-of-order
receipts are testable. Simulation never emits real SMS.

## v0.5.0 — Provider fidelity and production switching

- Expand Twilio compatibility based on versioned contract fixtures.
- Add OVH and Vonage capture adapters with published support matrices.
- Provider-neutral sending helper with local and real-provider drivers.
- Provider error formats, signatures, sender constraints and Unicode behavior.
- Generated API types/client examples and integration guides by framework.

Acceptance: the same sample application changes configuration between local and
one real provider; opt-in credentialed smoke tests record what local emulation
cannot guarantee. No production credentials are needed for normal CI.

## v0.6.0 — Native SMS relay

- Explicit capture/simulate/relay mode selection, clearly labeled UI.
- First real provider connector and Android SIM gateway proof of concept.
- Authenticated gateway enrolment, job leases, receipts and disconnect recovery.
- Recipient routing, configured sending limits, idempotency/reconciliation.
- WebOTP body builder and Android Retriever hash format validation.

Acceptance: an actual SMS arrives in a separate test phone's native Messages
app and a compatible app/browser offers the OTP. Validate on physical hardware;
do not label browser injection as a native SMS test. Document platform, SIM and
carrier conditions. Gateway hardware and real send cost remain optional.

## v0.7.0 — Device lab

- Android Emulator connector through an explicitly selected ADB serial.
- Emulator tooling preflight and local-only injected SMS history.
- Android gateway hardening, multi-SIM selection where supported, health view.
- USB modem/SIM connector after testing identified modem models.
- Samples for Android SMS Retriever/User Consent and iOS `.oneTimeCode`.
- Optional HTTPS Web Push with delivery/permission diagnostics.

Acceptance: repeatable emulator and physical-device test matrices, including
timeout, permission denial and manual-entry fallback. Never promise universal
SMS Retriever behavior on every emulator image or silent iOS inbox insertion.

## v0.8.0 — Self-hosted teams

- Scoped API keys, user sessions, project/inbox roles and audit events.
- PostgreSQL adapter, durable workers and backup/migration tooling.
- Scope-aware quotas, retention policies, usage metrics and deployment guide.

Acceptance: isolation tests cover every API, export, subscription and worker;
local single-user startup remains one command with SQLite.

## v0.9.0 — Hosted private beta

- Organization control plane, invitations, OIDC and managed inboxes.
- Billing metering, plan quotas, retention and support tooling.
- Optional authenticated sharing from local development.
- Operational monitoring, recovery drills, tenant deletion/export and status page.

Acceptance: provision an organization, isolate its traffic, restore from backup
and enforce quotas under load. Hosted infrastructure does not gate local use.

## v1.0.0 — Stable platform

- Stable v1 API and migration policy, supported-provider/device matrix.
- Benchmarked release budgets, accessibility review, published compatibility suite.
- Reliable local, self-hosted and hosted onboarding; public documentation website.
- Signed releases/provenance and sustainable maintainer/contributor process.

## Definition of done for every version

1. Acceptance journey works, and UI labels match implemented behavior.
2. Relevant unit, integration, SDK and browser checks pass.
3. Changes include documentation, migration notes and CHANGELOG entry.
4. `VERSION`, frontend package and lockfile versions agree.
5. Review status/diff, commit intended files, annotate `vX.Y.Z`.
6. Push the reviewed commit/tag when the remote is configured; tag CI reruns
   verification before publishing binaries and the multiarchitecture image.
7. Install one produced artifact and record smoke-test results.
