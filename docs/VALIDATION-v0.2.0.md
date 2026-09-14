# v0.2.0 validation

- Race-enabled Go integration tests pass for v1→v2 migration, preserved messages,
  concurrent single-use claims, expiry, persistence, read-only scope enforcement,
  SSE filtering and immediate revocation.
- Six Chromium checks pass across desktop and mobile viewports: capture workflow,
  official Twilio SDK, and QR-link → device registration → live SMS → reload →
  desktop revocation → cleared phone view.
- No real phone, carrier or native SMS autofill was exercised. QR rendering and
  link navigation are tested; optical scanning on hardware remains a manual check.
- SSE is an invalidation transport, not delivery-event history. A full resync is
  tested on reconnect; background push and offline caching are not implemented.
- Frontend assets are approximately 88.8 kB gzip in the production build.

v0.1.0's remote release workflow completed successfully, producing all five
binary archives and checksums: https://github.com/fless-lab/TextDock/releases/tag/v0.1.0
