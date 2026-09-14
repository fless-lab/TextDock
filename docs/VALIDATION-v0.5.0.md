# v0.5.0 validation

- SDK unit tests use independent local HTTP receivers to verify local/Twilio
  payloads and authentication, Vonage text/Unicode selection and HTTP-200 error
  handling, and OVH time synchronization/exact request signing.
- A public TypeScript contract fixture checks supported options, result types
  and expected compile errors for invalid calls.
- End-to-end tests run all four SDK drivers through TextDock capture adapters
  and verify their source normalization. Provider authentication and unsupported
  OVH batches are covered by Go race tests.
- The Node SDK has no third-party runtime dependencies. Packaging includes
  implementation, declarations, README and MIT license, excluding test fixtures.
- Existing simulation, callback-signature, pairing, workspace and accessibility
  journeys remain in the browser suite.

v0.4.0 remote CI completed successfully. Live credentials, paid sends and physical
native SMS/autofill are not part of these protocol tests. Capture adapters are
explicit subsets; see PROVIDERS.md rather than assuming full vendor emulation.
