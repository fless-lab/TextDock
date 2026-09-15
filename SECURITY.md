# Security

TextDock is a local/self-hosted developer tool. Anyone holding the server
token has full read/write access. v0.2 phone sessions use separate hashed
credentials with enforced recipient/run scope, read-only routes, 30-day expiry
and immediate revocation. Plain desktop query filters are not access controls.
Hosted multi-tenant deployment requires the later organization-scoped architecture.

The v0.8 preview adds project/inbox machine credentials with separate read,
capture/edit and delete permissions. API secrets have 256 random bits and are
stored only as SHA-256 hashes; scope and permissions are immutable. An explicit
route allowlist denies new operator endpoints by default. IDs, bulk operations,
exports, OTP waits and live streams are checked against the key's scope. Metadata
edits need read as well as write because their response includes message content.
Keys cannot create phone/push/gateway sessions or initiate relay/simulation jobs.

Revoked/expired keys fail authentication; active OTP waits and streams recheck
their credential. Already admitted requests can finish. Restoring a database
snapshot also restores its key/revocation state. See [API keys](docs/API-KEYS.md).
Anonymous loopback mode remains operator access: set the server token to enforce
isolation on a shared instance. Scoped keys are not a hosted tenant boundary.

The v0.6 preview adds separately scoped gateway credentials and an opt-in real
SMS relay. Gateway tokens are stored as hashes, cannot authorize the desktop API,
and can acknowledge only their own leased jobs. Provider credentials remain in
server environment variables. Idempotency key fingerprints survive message
deletion to prevent an old key from creating another carrier send. Uncertain
dispatches are not automatically retried.

Optional Web Push stores its generated VAPID key pair and browser subscription
parameters in SQLite. Treat database snapshots as sensitive. Only generic alerts
are encrypted and sent: never SMS bodies, numbers, OTPs or device credentials.
The phone worker stores a non-secret active-session binding and expiry, not the
device token or cached inbox contents. Session revocation cancels queued alerts;
already accepted platform deliveries cannot be recalled reliably.

Phone credentials may change their own notification preferences. Endpoint hosts
are restricted to supported HTTPS push services or explicit operator additions;
these routes do not grant arbitrary HTTP forwarding or access to other inboxes.

Default binding is loopback. Non-loopback binding requires a token with at least
16 characters. Same-origin browser checks, bounded request bodies and loopback
Host validation protect the local API boundary. Tokens are not accepted in query
strings and message contents are not included in routine request logs.

Messages and OTPs are stored unencrypted in SQLite until individually deleted
or the data directory is removed while the server is stopped. Filesystem access
and backups therefore have access to the contents. v0.3 offers opt-in automatic
retention; it is disabled by default. Storage is not encrypted. Use synthetic
data for development.

For remote connections, put TextDock behind HTTPS and an appropriate access
boundary. A shared-token local deployment is not a public SaaS installation.

Enable **Private vulnerability reporting** in
its Security settings. Report vulnerabilities through that private channel;
avoid posting working tokens or private messages in public issues. A maintainer
contact and supported-version policy must be published before a public launch.
