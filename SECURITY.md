# Security

TextDock is a local, single-workspace developer tool. Anyone holding the server
token has full read/write access. v0.2 phone sessions use separate hashed
credentials with enforced recipient/run scope, read-only routes, 30-day expiry
and immediate revocation. Plain desktop query filters are not access controls.
Hosted multi-tenant deployment requires the later organization-scoped architecture.

The v0.6 preview adds separately scoped gateway credentials and an opt-in real
SMS relay. Gateway tokens are stored as hashes, cannot authorize the desktop API,
and can acknowledge only their own leased jobs. Provider credentials remain in
server environment variables. Idempotency key fingerprints survive message
deletion to prevent an old key from creating another carrier send. Uncertain
dispatches are not automatically retried.

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
