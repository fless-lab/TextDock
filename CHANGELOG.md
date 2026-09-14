# Changelog

All notable changes are documented here. Versioning follows SemVer; before
v1.0, minor releases may evolve contracts with explicit migration notes.

## [Unreleased]

## [0.6.0-beta.2] - 2026-09-14

### Fixed

- Clear stale uncertainty diagnostics when a signed receipt confirms the real relay outcome.

### Added

- Direct Android development APK links in the relay UI and versioned downloads page.

## [0.6.0-beta.1] - 2026-09-14

### Added

- Explicit, default-disabled real SMS relay with a built-in Go Twilio transport.
- Persistent dispatch state, idempotency keys, admission/dispatch limits, recipient allowlists and queue TTL.
- Signed and correlated Twilio receipts; uncertain sends are recorded without automatic retransmission.
- Android gateway enrolment, hashed credentials, owner-bound leases, revocation and idempotent results.
- Android development companion APK using the default SIM, foreground service and multipart submission callbacks.
- Relay UI, gateway management, real-send action labels and dispatch inspection.
- WebOTP and Android Retriever message-format helpers, CLI and SDK relay options.
- Tests for opt-in, idempotency conflicts, expiry, quotas, receipt ordering and gateway authorization.

### Prerelease scope

- Android build and lint pass; physical SMS receipt, native autofill, OEM behavior and carrier restrictions still need device testing.
- The development APK is debug-signed. Stable GitHub Pages remains on v0.5.2.
- Database schema 5 adds relay state and idempotency tombstones; back up before upgrading.

## [0.5.2] - 2026-09-14

### Changed

- Replaced the promotional website homepage with a concise project page focused on installation, API usage and documentation.
- Standardized website typography on Arial/Helvetica, with conventional sizes and weights.
- Removed hero slogans, feature grids, terminal-window decoration and promotional calls to action.

## [0.5.1] - 2026-09-14

### Added

- Public GitHub Pages website with a restrained responsive landing page, downloads and full documentation.
- Local documentation search, dark/light themes, API-contract downloads, canonical URLs and sitemap.
- Single-source guide generation from repository Markdown, with checked links and anchors.
- Website build, browser and accessibility checks in CI; automatic Pages deployment after stable releases.
- Clear current-status/next-steps overview and explanation of test phone numbers versus real SMS numbers.

### Distribution

- The website tracks the latest stable release; prereleases do not replace it.
- Application and SDK behavior remain the v0.5 feature set.

## [0.5.0] - 2026-09-14

### Added

- Dependency-free Node ESM SDK with TypeScript declarations and local/Twilio/Vonage/OVH drivers.
- Environment-based provider switching with one send interface and explicit acceptance semantics.
- OVH server-time synchronization/request signing and Vonage GSM/Unicode selection.
- Bounded response handling, normalized provider errors and uncertain-outcome reporting without automatic send retries.
- Vonage and OVH single-recipient capture adapters with declared compatibility matrices.
- SDK protocol fixtures and end-to-end tests through all four capture paths.
- Versioned SDK tarball in GitHub Releases, included in SHA256SUMS.

### Scope

- Provider routes still capture/simulate locally. Non-local SDK drivers can send
  real SMS when configured with valid credentials; carrier integration was not exercised by CI.
- No schema migration is required beyond v0.4's schema version 4.

## [0.4.0] - 2026-09-14

### Added

- Inbox-scoped seeded scenarios for delayed delivery, failures, expiry and HTTP rejection.
- Persistent message lifecycle history and incoming SMS simulation.
- Transactional outbox with ordered transitions, recoverable leases and stale-worker fencing.
- JSON/HMAC-SHA256 and Twilio/HMAC-SHA1 callbacks, bounded retries and manual replay.
- Request/response inspection, status filtering and CLI simulation options.
- Scenario editor and message Events tab with live updates.
- Crash/restart, signature, retry/replay, rejection and cascade-deletion tests.

### Changed

- Capture orchestration is now an application service, independent of HTTP handlers.
- Database schema v4 stores lifecycle events, scenarios, jobs and callback attempts.
- `/api/v1/info` reports `mode: local`; each message identifies capture versus simulation.
- Twilio `StatusCallback` is supported when a simulation scenario is explicitly selected.

## [0.3.0] - 2026-09-14

### Added

- Projects and inboxes with backwards-compatible `local` capture defaults.
- Stable cursor pagination, recipient/run/tag/date filters and server-side OTP/favorite views.
- Favorite editing, tags, recipient conversation filter, batch deletion and CSV page export.
- CLI send/list/wait/purge, streaming JSON/JSONL/CSV export and SQLite backup/restore.
- Strict JSON configuration, opt-in retention and custom RE2 OTP extraction.
- Unicode-trigger diagnostics, command palette and dependency-free test helper.
- v2→v3 migration for projects and inbox-bound phone credentials.
- Tests over 100,000 messages, snapshot integrity, CLI workflows and cross-inbox phone isolation.

### Fixed

- In-memory databases now survive connection recycling after request cancellation.
- SQLite foreign-key enforcement is restored on every replacement connection.

### Upgrade

- Existing messages and paired devices belong to the `local` inbox after migration.
- Back up before upgrading. Older binaries cannot safely manage the new workspace schema.

## [0.2.0] - 2026-09-14

### Added

- One-use QR phone pairing with two-minute challenges and 30-day device sessions.
- Read-only phone credentials scoped to a recipient and optionally a test run.
- Dedicated `/phone` conversation UI, persistent device login and disconnect.
- Live SSE invalidations, automatic reconnect/resync and immediate revocation.
- Connected-device management, LAN address suggestions and `--public-url`.
- Phone install manifest and clipboard fallback for same-LAN HTTP connections.
- Transactional v1→v2 database migration preserving captured messages.
- Integration/browser tests for pairing replay, isolation, concurrency and revocation.

### Changed

- Desktop inbox uses live events instead of two-second polling.
- Docker builds UI on the builder architecture, avoiding emulated Node builds.

### Upgrade

- Existing databases migrate automatically. Back up before upgrading; v0.1
  cannot read device sessions. Physical native SMS delivery remains a later relay feature.

## [0.1.0] - 2026-09-14

### Added

- Single-binary Go server with embedded React/TypeScript interface on port 18257.
- SQLite persistence, JSON capture API, literal search and recipient/run/time filters.
- Responsive inbox, dark/light themes, local test composer, JSON export and deletion.
- GSM-7/UTF-16 inspection with segment-boundary handling and OTP suggestions.
- Bounded OTP wait API with required recipient and test-run filters.
- Twilio Messages create subset, tested through the official Node SDK.
- Shared-token LAN access and phone connection guidance.
- Architecture and v0.1–v1.0 roadmap for pairing, simulation, relays and hosting.
- OpenAPI contract, native autofill documentation and WebOTP integration example.
- Backend/browser tests, Docker packaging and tag-triggered release automation.

### Scope

- Capture only: no real SMS, QR pairing, push, delivery callbacks or hosted accounts.
- Inbox shows the latest 100 matches; API limit is 200; no automatic retention yet.
- Phone support is a responsive browser interface, not native SMS injection.
