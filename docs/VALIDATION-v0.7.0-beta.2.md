# v0.7.0-beta.2 validation

## Local checks — 2026-09-15

- Go formatting, vet and all race-enabled tests passed, including the existing
  100,000-message storage coverage and the new notification lifecycle tests.
- All 5 SDK tests and strict TypeScript checks passed.
- All 24 application browser tests passed across desktop and mobile projects.
- All 8 website browser/accessibility tests passed; the site build and local
  link/anchor checker passed.
- All 8 OpenAPI contracts, GitHub workflow lint and release metadata checks passed.
- The CGO-free binary and Docker image built successfully. The Docker server
  reported `v0.7.0-beta.2` and returned a healthy response.
- Binary smoke checks passed in an isolated container network: default port,
  embedded UI, capture/OTP, bind conflicts and network token policy. This avoided
  interfering with an existing local server on port 18257.
- Built application JS and CSS total approximately 100 kB gzip.

## Web Push coverage

The protocol tests receive and independently decrypt an `aes128gcm` request over
TLS, verify its VAPID JWT and check that SMS contents, OTPs and numbers are absent.
Storage tests cover scoped enqueue, coalescing, persisted keys, revocation,
expiry, subscription transfer and stale delivery responses.

Browser tests use isolated persistent Chromium profiles. PushManager registration
is synthetic; service-worker push events are injected through Chromium's debugging
protocol. Tests exercise generic alert display, session invalidation, permission
denial, manual pairing and a message-free offline fallback.

## Release checks and platform boundary

The tag workflow gates publication on reusable CI, cross-platform builds,
multiarchitecture container publishing, SDK packaging, Android build/lint and a
disposable Android emulator integration test. Its run and downloadable artifacts
provide the publication evidence separately from these local results.

Real FCM/APNs/browser-platform delivery to a physical locked phone and iOS
home-screen installation/storage behavior have not been verified. Carrier SMS,
native autofill and physical multi-SIM/OEM coverage remain separate manual checks.
This is a prerelease; the public documentation site remains on stable v0.5.2.
