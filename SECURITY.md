# Security

v0.1 is a local, single-workspace developer tool. Anyone holding the server token
has full read/write access. Test run IDs and recipient filters are not access
controls. Hosted multi-tenant deployment requires the later scoped architecture.

Default binding is loopback. Non-loopback binding requires a token with at least
16 characters. Same-origin browser checks, bounded request bodies and loopback
Host validation protect the local API boundary. Tokens are not accepted in query
strings and message contents are not included in routine request logs.

Messages and OTPs are stored unencrypted in SQLite until individually deleted
or the data directory is removed while the server is stopped. Filesystem access
and backups therefore have access to the contents. Automatic retention and
encrypted storage are not v0.1 features. Use synthetic data for development.

For remote connections, put TextDock behind HTTPS and an appropriate access
boundary. A shared-token local deployment is not a public SaaS installation.

Once the GitHub repository exists, enable **Private vulnerability reporting** in
its Security settings. Report vulnerabilities through that private channel;
avoid posting working tokens or private messages in public issues. A maintainer
contact and supported-version policy must be published before a public launch.
