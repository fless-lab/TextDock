# v0.8.0-beta.3 validation

## Local checks — 2026-09-16

- Go formatting/vet and the complete race-enabled suite passed, including the
  existing 100,000-message storage checks with audit recording enabled.
- All 5 SDK tests, frontend formatting/types and SDK strict type checks passed.
- Embedded UI/binary builds and all 32 desktop/mobile browser tests passed.
- All 8 website browser/accessibility tests, site generation and local link/anchor
  checks passed.
- All 11 OpenAPI contracts, workflow lint and release/native metadata checks passed.

## Audit-specific coverage

- Injected audit-insert failure rolls back membership changes and their session
  revocations, password changes, API-key revocation, message capture/update/delete,
  purge/retention and project/inbox creation.
- Successful message history survives message deletion and a database restart.
- Repeating message deletion/key revocation does not duplicate committed events.
- Descending event-ID pagination does not overlap pages.
- HTTP events identify the authenticated user/key/operator; failed password checks
  use an anonymous actor without the submitted username.
- Serialized events exclude tested password/token/hash, SMS body, OTP, number,
  sender, test-run and tag canaries.
- Project admins cannot query instance/other-project history or move an inbox
  filter outside their chosen project. Viewers and machine keys are denied.
- Export events distinguish prepared data from completed mutations.
- Browser filters, retained deleted-message history, metadata-page download and
  audit dialog accessibility pass on desktop and mobile.

## Preview boundary

SQLite schema 11 adds retained audit events. This is a closed metadata journal,
not a complete HTTP access log or a signed/tamper-evident ledger. Read/rejected
events use separate bounded appends; committed resource events share the mutation
transaction. SMS retention does not remove audit rows, and there is no audit
deletion/automatic retention API in this preview.

The release workflow also gates publishing on container/cross-platform builds,
Android/iOS sample tests and the disposable SMS emulator integration. Physical
carrier/autofill/push checks remain separate. Pages remains on stable v0.5.2.
