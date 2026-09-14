# v0.1.0 local validation

Verified on Linux amd64, 2026-09-14, using Go 1.26.8 and Node.js 24.20.0.

| Check | Result |
|---|---|
| Go formatting, vet, race-enabled domain/storage/HTTP tests | Passed |
| TypeScript checking, Prettier, Vite production build | Passed |
| Desktop Chromium capture → inspect OTP/JSON → search → delete | Passed |
| Mobile Chromium viewport, same journey and horizontal overflow check | Passed |
| Official Twilio Node SDK create contract | Passed |
| Built binary, default port 18257, embedded UI, capture and OTP | Passed |
| Occupied port failure and non-loopback token requirement | Passed |
| Docker Linux amd64 build, capture and persistence across restart | Passed |
| Cross-compilation: Linux arm64, macOS amd64/arm64, Windows amd64 | Passed |
| GitHub workflow static validation with actionlint | Passed |
| OpenAPI schema validation with Redocly | Passed |
| Tag/package/lockfile/OpenAPI/changelog version consistency | Passed |

Frontend production JS+CSS: approximately **79.5 kB gzip** according to Vite.
This is a build-size measurement, not an end-to-end performance benchmark.

No physical phone, mobile carrier, native OS autofill or remote GitHub release
was exercised. Mobile browser emulation is not physical-device verification.
Cross-compiled artifacts were not run on macOS, Windows or ARM hardware.

`origin` is `https://github.com/fless-lab/TextDock.git`; it was empty when checked.
GitHub CLI authentication was unavailable. The tag workflow is configured and
statically validated but needs a remote tag push to execute on GitHub.
