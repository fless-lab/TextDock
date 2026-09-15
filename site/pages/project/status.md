# Current status and next steps

**Current release: {{VERSION}}.** TextDock is a usable local SMS development tool.
The complete native-device and hosted-service roadmap is not finished.

## Available today

| Area | Implemented |
|---|---|
| Local development | Standalone binary, embedded UI, SQLite, Docker and configurable port 18257 |
| Inbox workflow | Projects, search, pagination, tags, favorites, exports and CLI |
| Phone access | QR pairing, scoped read-only sessions, live updates and revocation |
| Phone notifications | Installable inbox and optional generic encrypted Web Push, with diagnostics |
| Native-device preview | Twilio/Android relay, explicit SIM selection, gateway health and Android emulator injection |
| Testing | OTP detection/wait API, seeded scenarios, simulated incoming SMS and lifecycle history |
| Webhooks | Signatures, persistent retries, request/response inspection and replay |
| Integration | JSON API, declared Twilio/Vonage/OVH capture subsets, Node sender SDK |
| Distribution | GitHub releases, multiarchitecture containers, checksums, SDK package and this documentation site |

“Phone access” currently means the TextDock browser inbox. It does not insert a
message into a physical phone's native Messages application.

## v0.6 preview — Native SMS relay

The integrated relay includes a Twilio connector and Android SIM gateway,
device enrolment, durable jobs and receipts, explicit sending modes and
reconciliation of uncertain sends. Physical validation remains outstanding.

**Acceptance:** a real SMS reaches a separate test phone and a compatible app or
browser offers the OTP. This must be checked on physical hardware. Existing SDK
drivers can make provider requests, but live carrier delivery and native autofill
have not been verified by the local automated suite.

## v0.7 preview — Device lab and phone notifications

Explicit Android emulator selection/injection, gateway health/SIM selection and
optional HTTPS push notifications are implemented. Tested USB modems, native
Android/iOS examples and physical/background-delivery checks remain next steps.

## Then: v0.8–v0.9 — Teams and hosted service

Scoped API keys and user roles, PostgreSQL, workers and audit logs come before
organizations, invitations, quotas and managed hosting. The public GitHub Pages
site is documentation; it is not the future SMS hosting service.

Local use will remain independent of a hosted account.

## Stability and verification

TextDock is pre-1.0. Compatibility is documented per operation, rather than
claiming complete vendor emulation. The test suite covers local behavior,
provider protocol fixtures, phone-browser scopes, retries and migrations.
Hardware, carrier delivery and manual assistive-technology checks are separate
verification activities.

See the [detailed roadmap](/project/roadmap), [provider support matrix](/guide/providers)
and [native SMS guide](/guide/mobile-and-otp).
