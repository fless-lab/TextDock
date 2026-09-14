# v0.4.0 validation

- Go race tests exercise transactional scheduling, ordered transition recovery
  after a claimed job survives restart, stale-worker fencing, seeded repeatability
  and rejection before persistence.
- Callback tests cover inbound STOP, HMAC verification, 503→204 retry timing,
  durable attempt diagnostics, manual replay and cascading removal of callback data.
- Chromium tests create a scenario through the UI, deliver a simulated message,
  inspect queued/sent/delivered events and replay a receipt. The receiving test
  server verifies signatures using the official Twilio Node SDK.
- Existing desktop/mobile capture, pairing, workspace and accessibility checks
  remain in the suite. The new simulation journey passes on both viewports.
- Migration v3→v4 adds persistent histories, scenarios, jobs and attempts.
- No carrier SMS, physical Android/iOS autofill, or production provider credentials
  are involved. Callbacks are exercised against local HTTP receivers.

v0.3.0 remote release CI completed successfully. The installed frontend remains
under 100 kB gzip for its JavaScript and CSS.
