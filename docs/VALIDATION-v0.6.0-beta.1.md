# v0.6.0 beta validation

- Real-relay paths are opt-in; capture remains the default and the UI action is
  explicitly labelled “Send real SMS”. Automated tests use local provider fakes
  and a protocol-only gateway, not a carrier.
- Go tests cover request idempotency/conflicts, deletion tombstones, unknown
  outcomes without resending, admission/dispatch limits, queue TTL, owner-bound
  gateway results, revocation, signed receipts and non-regressing delivery state.
- Browser tests exercise gateway enrolment, OTP formatting, a queued real-mode
  intent, a fake gateway acknowledgement and the dispatch history view.
- The Android development app builds and passes Android Lint in GitHub Actions.
  It uses platform SMS APIs with runtime permission checks and a foreground
  service, persisting uncertain/pending results instead of repeating sends.
- WebOTP/Android format tests check hostname/code/hash syntax. They do not prove
  autofill behavior on a physical device.

## Not performed

No physical Android/iOS device, real SIM or production provider account was used.
Carrier delivery, OEM background/permission behavior, default-SIM selection,
multipart physical reception and native OTP suggestions require the manual
matrix in RELAY.md before a stable v0.6 claim.

The stable documentation website was redesigned and published as v0.5.2, using
Arial/Helvetica and direct technical documentation rather than promotional layout.
