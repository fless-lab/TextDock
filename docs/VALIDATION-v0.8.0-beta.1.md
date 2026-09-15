# v0.8.0-beta.1 validation

## Local checks — 2026-09-15

- Go formatting/vet and all race-enabled tests passed, including the existing
  100,000-message storage coverage and the new credential/isolation tests.
- All 5 SDK tests, frontend formatting/types and SDK strict type checks passed.
- The embedded UI/binary build and all 26 desktop/mobile browser tests passed.
- The website build, local link/anchor checker and all 8 website browser and
  accessibility tests passed.

## API key isolation coverage

- Inbox defaults and project grants, including a newly created project inbox.
- Cross-scope read/export/OTP requests and message ID/event access.
- Independent read/write/delete permissions; metadata edits require read too.
- Mixed-scope bulk deletion is rejected before modifying any message.
- Denial of key/workspace administration, pairing, push, relay, gateway, lab,
  simulation and future/unrecognized operator routes.
- Twilio Basic, Vonage secret and OVH consumer-key credentials, including invalid,
  read-only and device credentials on an otherwise anonymous local server.
- Case-insensitive Bearer handling; unsupported Basic API authentication cannot
  fall back to local operator access.
- Persisted hashes, restart behavior, expiration and invalid grants.
- Scoped SSE message invalidations and revocation of streams/pending OTP waits.
- Browser key issuance, one-time display, capture into the selected inbox,
  forbidden cross-scope reads, secret disappearance on close and revocation.
- Automated accessibility checks for the key management dialog.

## Preview boundary

These are machine credentials, not user accounts or hosted tenant identities.
Operator access remains available without a credential in zero-configuration
loopback mode; set the server token to enforce access control on shared instances.
Already admitted requests may finish after revocation.

SQLite schema 9 adds API keys. Backups retain credential hashes and their recorded
revocation state. Release CI also checks cross-platform binaries, containers,
Android/iOS sample builds/tests and the disposable SMS emulator integration.
Physical carrier/autofill/push validation remains separate. Pages stays on v0.5.2.
