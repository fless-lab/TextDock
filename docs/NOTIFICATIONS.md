# Phone installation and Web Push

The phone inbox can be installed on the home screen and receive **generic**
message alerts. Notifications contain no SMS body, recipient, sender or OTP.
Opening an alert returns to the paired inbox, where normal session authorization
is still required.

Web Push is optional and **disabled by default**. Foreground inbox updates continue
to use the local SSE connection without a push account or external service.

## Server setup

Serve TextDock through a trusted HTTPS origin and configure:

```sh
TEXTDOCK_PUBLIC_URL=https://textdock.example.test \
TEXTDOCK_TOKEN='your-long-local-token' \
TEXTDOCK_PUSH_ENABLED=true \
TEXTDOCK_PUSH_SUBJECT=mailto:developer@example.test \
textdock
```

Use your actual server hostname and contact address. The contact can also be an
HTTPS URL; it defaults to `TEXTDOCK_PUBLIC_URL`. A persistent VAPID key pair is
generated automatically in SQLite. No push-service account or manually generated
key pair is required. Keep the database when restarting to preserve subscriptions.

The HTTP server itself still listens on port 18257. For example, a local Caddy
reverse proxy can terminate TLS:

```text
textdock.example.test {
    tls internal
    reverse_proxy 127.0.0.1:18257
}
```

The phone must resolve the hostname to your computer and trust the proxy's CA.
Do not use an untrusted certificate or plain HTTP LAN IP for Web Push. Loopback
HTTP is accepted for desktop development tests, with an explicit contact email;
it is not a way to make a phone's remote LAN connection secure.

## Pair and install

1. Pair the phone from the desktop, using the QR code or a copied pairing link.
2. Open **Notifications & installation** in the phone inbox.
3. Install using the browser prompt, or its **Add to Home Screen** command.
4. Press **Enable notifications** and grant the browser permission.
5. Use **Send test notification** and inspect the status panel.

On iPhone/iPad, Web Push requires a supported OS/browser and an installed
home-screen app (iOS/iPadOS 16.4+). Install from Safari and open the installed app
before enabling notifications. Its storage may be separate from the Safari tab:
the phone page accepts a fresh pairing **code or full link** so you can pair from
inside the installed app. No notification permission is requested automatically.

Private/incognito browser profiles may prohibit notifications. If permission is
denied, change the site's browser setting and refresh the diagnostic panel.
An unsupported browser can still use the foreground inbox.

## What is stored and sent

- SQLite holds the VAPID key pair, subscription endpoint/encryption parameters,
  a short-lived job queue and the latest diagnostic for each device.
- A push subscription is bound to a paired device's inbox/recipient/run scope.
  Devices can update their own notification preferences, not other sessions or
  application messages.
- New messages enqueue scoped notifications in the same transaction as capture.
  Metadata edits and ordinary SSE refreshes do not generate additional alerts.
- Pending alerts coalesce per device. Queued jobs expire after 60 seconds and
  transient push-service failures retry at most three attempts.
- Payloads are encrypted using Web Push `aes128gcm`; requests authenticate with VAPID. The
  decoded payload contains only the device ID, subscription generation and alert
  kind. The worker always uses fixed notification text and opens `/phone`.
- The service worker stores only its active session binding, generation and
  expiration in IndexedDB. It does not store the device token or cache SMS/API
  responses. Its offline page contains no messages.

The browser/platform push service and Internet connectivity are required for
background delivery. Returning to a privately hosted inbox may still require
Wi-Fi/VPN; the offline page explains how to reconnect.

## Subscription lifecycle

Disabling notifications removes the server subscription/jobs, unbinds the worker,
closes displayed TextDock notifications and unsubscribes the browser. Disconnect,
re-pairing, session expiry and desktop revocation also invalidate the relevant
delivery state. The worker checks the session and subscription generation before
showing an alert; a queued payload for an old pairing is not shown as a new one.

A browser has one push subscription for this phone worker. Re-pairing to another
scope transfers that subscription only when its endpoint encryption keys agree,
and removes old pending jobs. A 404/410 from the platform invalidates a stale
subscription; enabling again obtains a new one.

Already accepted platform deliveries cannot be recalled reliably. Notifications
are deliberately generic, and device display is not inferred from a successful
server HTTP response.

## Diagnostics and endpoint policy

The panel reports browser permission, server enablement, registration state,
queued work and the most recent push-service HTTP outcome. It refreshes while
expanded. `accepted` means the push service accepted the alert, not that the
phone displayed it. A test notification does not create or deliver an SMS.

Default endpoint hosts cover FCM, Mozilla Push, Apple Web Push and Windows Push.
Endpoints must be HTTPS; redirects are not followed. For an operator-controlled
service, `TEXTDOCK_PUSH_ALLOWED_HOSTS` adds exact trusted hosts. This is not a
device-controlled arbitrary HTTP forwarding endpoint.

## Verification scope

Go tests decrypt a real encrypted request at a local TLS receiver and verify its
VAPID signature and generic contents. They cover scope isolation, coalescing,
expiry, revocation, session transfer and stale response handling.

Browser tests use isolated persistent profiles, synthetic PushManager registration
and Chromium's service-worker push delivery tooling. They verify notifications,
privacy, revoked sessions, denied permissions and offline fallback. Physical
locked-screen delivery, iOS installation/storage behavior and provider-specific
device behavior remain manual platform checks; they are not claimed by these tests.
