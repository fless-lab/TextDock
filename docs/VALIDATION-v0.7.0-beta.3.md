# v0.7.0-beta.3 validation

## Native sample checks — 2026-09-15

- Android gateway/sample compilation and lint passed with Java 17, Gradle 8.9 and
  Android SDK 35. The sample uses the Play services API version compatible with
  the existing AGP lint toolchain and ContextCompat receiver registration.
- Android sample unit tests cover exact ASCII code extraction, ambiguous input,
  a signing-hash vector and cancelled/expired attempt generations.
- Android emulator UI tests cover manual entry, invalid input, cancellation,
  empty input after recreation and local-only confirmation.
- iOS Xcode build, unit tests and simulator UI test passed on the macOS runner.
  Tests cover exact input policy, expiry, cancellation, cleared input and a fresh
  attempt after cancellation.

Pre-release evidence:

- [Android build and tests](https://github.com/fless-lab/TextDock/actions/runs/34917123373)
- [iOS build and tests](https://github.com/fless-lab/TextDock/actions/runs/34916942661)

## Application and documentation checks

- Go formatting/vet, frontend formatting/types and SDK strict type checks passed.
- Embedded UI/binary build and all 24 application browser tests passed.
- Website build, local link/anchor validation and all 8 website browser and
  accessibility tests passed.
- Release/native version metadata, all 8 OpenAPI contracts and workflow lint passed.
- The reusable release workflow also gates publishing on the full Go race/SDK
  suite, binary smoke, cross-compilation, container build and SMS lab integration.

## Artifacts and verification boundary

The release includes the Android receiving sample APK and iOS simulator app zip
alongside the gateway APK, binaries and SDK. Native sample artifacts are covered
by the release checksums. A physical iPhone requires a separately signed Xcode
build; the simulator archive is not an installable iPhone IPA.

The automated native form tests **type** codes. They do not fabricate trusted
Google SMS broadcasts or claim actual consent/keyboard autofill. Physical SMS
receipt, Retriever/User Consent behavior, signing variants and iOS suggestions
remain manual platform checks described in [the sample guide](NATIVE-SAMPLES.md).

The samples do not verify an OTP against an authentication backend. Database
schema remains 8. This prerelease leaves the public site on stable v0.5.2.
