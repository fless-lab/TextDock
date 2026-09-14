# v0.7.0 device-lab validation

- Go tests cover emulator eligibility, physical/offline/unbooted rejection,
  bounded command output, unsafe-control rejection, explicit target arguments,
  UTF-16 console escapes, persisted history, scope isolation, idempotency and
  recovery of interrupted injections.
- The first real-emulator test found non-BMP truncation in raw `sms send` input.
  The legacy PDU console path also acknowledged commands without guest receipt
  on the tested modem. The supported text path now uses explicit UTF-16 escapes.
- A disposable Android 35 Google APIs emulator verified the exact inbox content
  with accents, Chinese characters, emoji, quotes, backslashes and line breaks.
  Repeating the same keyed request did not create another SMS.
- Browser fixtures verify explicit selection, disabled physical targets, result
  inspection and failed injection history. They are distinct from the real
  emulator integration workflow.
- Gateway tests cover heartbeat recording, online/stale/revoked states and
  credential enforcement. The Android companion builds and passes Android Lint
  with SIM selection, optional phone-state permission and reset-generation logic.

## Still outside the verified matrix

No physical multi-SIM device, USB modem, real carrier delivery or physical
Android/iOS OTP autofill was tested. The emulator proof is SMS receipt in a
virtual device, not verification of those capabilities. Web Push and the broader
hosted/team roadmap are not implemented by this release.
