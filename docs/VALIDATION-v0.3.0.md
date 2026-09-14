# v0.3.0 validation

- Go race tests cover workspace isolation, custom OTP extraction, cursor
  boundaries, metadata updates, CLI workflows and snapshot restore refusal when
  the destination exists.
- A 100,000-message fixture verifies bounded first/next pages, indexed inbox
  ordering, literal search, complete snapshot integrity and retention cleanup.
- A regression test discards an in-memory request connection and verifies that
  messages and foreign-key enforcement survive replacement.
- Ten Chromium tests pass across desktop/mobile: existing capture and Twilio
  contracts, paired phone scope/revocation, project creation, favorite/tag editing,
  CSV download and batch deletion. Automated axe WCAG A/AA checks pass for the
  inbox and compose dialog in both viewports.
- Frontend assets remain approximately 91.4 kB gzip.
- OpenAPI contracts and release metadata validate; Docker build is checked before tagging.

The downloaded v0.2.0 Linux amd64 release artifact passed SHA256 verification and
the executable smoke test. v0.2's remote release CI completed successfully.
Physical-device scanning, native SMS delivery and manual assistive-technology
audits remain distinct checks, not implied by browser emulation or axe results.
