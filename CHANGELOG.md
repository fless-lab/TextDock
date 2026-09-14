# Changelog

All notable changes are documented here. Versioning follows SemVer; before
v1.0, minor releases may evolve contracts with explicit migration notes.

## [Unreleased]

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
