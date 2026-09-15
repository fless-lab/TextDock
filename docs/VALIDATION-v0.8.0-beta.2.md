# v0.8.0-beta.2 validation

## Local checks — 2026-09-15

- Go formatting/vet and the complete race-enabled test suite passed, including
  the existing 100,000-message storage tests.
- All 5 SDK tests, frontend formatting/types and SDK strict type checks passed.
- All 30 application browser tests passed across desktop/mobile layouts.
- All 8 website browser/accessibility tests, site generation and local
  link/anchor checks passed.
- All 10 OpenAPI contracts, workflow lint and native/application release metadata
  checks passed.

## Account and authorization coverage

- Random password salts, correct/incorrect verification, password/input policy
  and rejection of unbounded Argon2 parameters.
- Password-work concurrency and account attempt budgets.
- Project role selection without cross-project permission union.
- Viewer/member/admin capture, metadata, deletion, exports and OTP access.
- Cross-project message IDs and mixed-project batch rejection before mutation.
- Project-admin inbox/member operations and denial of account/key administration,
  other projects, relay/lab/phone/provider integration privileges.
- Self-session ownership, password changes, disabled accounts and generic login
  failures; account listings do not expose password material.
- Session persistence, expiry and stale-password login rejection after reset.
- Membership changes invalidate existing sessions and close their live streams.
- Browser account creation, role assignment, user login, role-aware controls,
  password change, logout and accessible team/account dialogs.
- A deliberately delayed capture response is released after switching to another
  user and must not repopulate the new user's inbox or detail view.

## Distribution and preview boundary

SQLite schema 10 adds accounts, memberships and hashed user sessions. The release
workflow gates publication on full CI, binary/container builds, Android/iOS sample
checks and the separate SMS emulator integration. Checksums and post-publication
artifact checks are separate from these source-suite results.

These are local password accounts with project roles, not hosted organizations.
The operator token is still required to enforce shared-instance authentication.
Sessions use tab-scoped Bearer storage and a 12-hour absolute lifetime. In-flight
requests may complete after revocation; already viewed data cannot be recalled.
Audit logs, OIDC/MFA, invitations and PostgreSQL remain later milestones.

Physical carrier SMS, autofill and background Web Push still require device
validation. The public documentation site remains on stable v0.5.2.
