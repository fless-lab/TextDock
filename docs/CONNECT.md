# Connect a phone

1. Start a LAN-accessible server with a token:

   ```sh
   TEXTDOCK_TOKEN='your-long-local-token' textdock --listen 0.0.0.0:18257
   ```

2. Unlock the desktop UI at `http://localhost:18257`.
3. Open **Open on phone**, choose the phone-facing origin and recipient, and
   optionally restrict access to one test run.
4. Create a pairing code and scan it. Enter a device name on the phone.
5. Send to that recipient; the phone updates immediately while connected.

Codes expire after two minutes and can be claimed once, atomically. Sessions
expire after 30 days and survive server/browser restarts. The database stores
SHA-256 credential hashes, not usable codes or device secrets. The phone keeps
its credential in local storage for repeat visits. “Disconnect this browser”
removes that browser's copy; **Revoke** on the desktop disables the server-side
session, closes active streams and clears the phone view.

Pairings are limited to one recipient, optionally a run. Device credentials only
work on the read-only `/connect/v1/` endpoints. They cannot access the desktop API,
delete messages or subscribe to another recipient. A shared desktop credential
still grants full workspace access. See [connection OpenAPI](../api/connect.openapi.yaml).

## Network setup

Native binary: private IPv4 interface addresses are suggested. Select the address
reachable from your phone; VPN, container and disconnected interfaces can also
appear. The service never automatically changes firewall or Wi-Fi settings.

Docker: publish the host port to the LAN (`18257:18257`) and set a custom token.
Set `TEXTDOCK_PUBLIC_URL=http://YOUR_COMPUTER_IP:18257` so QR codes use the host,
not Docker's internal network address. The shipped Compose mapping stays local.

HTTPS: terminate TLS using your preferred reverse proxy, preserve the public
Host header, disable response buffering for SSE, and set `TEXTDOCK_PUBLIC_URL`
to the trusted HTTPS origin. A phone must trust the certificate and resolve the
hostname. Keep the bearer token; HTTPS alone is not authorization.

The phone view includes a web manifest. Use the browser's **Add to Home Screen**
option where supported; install UX depends on browser and secure context.
Offline message caching and background push are not enabled. Suspending the page
may pause updates; resume/reconnect causes a fresh scoped read. Clipboard copy
falls back to a temporary selected field on HTTP; codes always remain selectable.

## SSE contract

Desktop: `/api/v1/events`; device: `/connect/v1/events`, both using authorization
headers. Events are `sync`, `heartbeat`, or `revoked`, with empty JSON data.
The first event is `sync`. Re-read the corresponding message endpoint after it.
Notifications are coalesced, not durable event history. Streams send heartbeats
every 15 seconds and bound each write so slow/disconnected clients are released.
An expired or revoked device must pair again. No SMS bodies are in broadcasts.
