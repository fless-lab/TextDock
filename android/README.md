# Android SIM gateway — development preview

The companion app polls an explicitly enabled TextDock Android relay and sends
claimed messages using the selected SIM or the phone's default SMS SIM. It uses only Android platform
APIs; there is no external Android runtime dependency.

For an application that **receives** an OTP, use the separate
[Android/iOS OTP samples](../docs/NATIVE-SAMPLES.md). The Android receiving module
uses Google Play services; it does not send SMS or connect to the gateway API.

## Setup

1. Run TextDock with `TEXTDOCK_RELAY_DRIVER=android`, a server token and LAN access.
2. Open **Relay** on the desktop and create a gateway credential.
3. Install the development APK from the prerelease on an Android 8+ test device
   with an active SIM and a configured default SMS subscription.
4. Enter the server's LAN/HTTPS origin and gateway token. Optionally press
   **Refresh SIM list** to grant phone-state permission and choose a specific
   subscription; default-SIM sending does not require that additional permission.
   Press **Start gateway** and grant SMS permission. A foreground notification
   indicates activity.
5. Use **Relay — real SMS** in the TextDock composer and address a separate phone
   you control. The SIM determines the sender; an alphanumeric `from` is not used.

The app reports `sent` after successful SMS submission callbacks for every part.
That does not prove delivery to the recipient. Mixed-part failures, missing
callbacks or interrupted service state are reported as `unknown`, never resent
automatically. The server retains the request state for inspection.

**Stop gateway** stops polling, but cannot undo an SMS already handed to Android.
Server-side credential revocation prevents further claims/results. Do not switch
servers, credentials or SIM selection while a result is pending; inspect that job
first. **Reset connection** stops the service and forgets local settings/pending
results. It never repeats a carrier send. A server job whose result was lost
becomes unknown and remains available for inspection.
An acknowledgement rejected as a missing/conflicting job clears the local pending
result and records a diagnostic; it does not issue another carrier send.

## SIM selection and connection status

A specific SIM is checked again before dispatch. If it is removed, inactive or
phone-state permission has been revoked, the job fails rather than silently
using another subscription. Subscription IDs can change when SIMs are replaced;
refresh and choose explicitly after a change.

The app reports a heartbeat every 15 seconds with its model, app version and
selected subscription ID. The Relay view shows the last report and considers it
online for 45 seconds. This indicates server connectivity, not radio readiness
or carrier delivery. Older servers without the heartbeat endpoint remain usable.

Connection generations and short synchronized state updates keep an old service
or SMS callback from writing into a reset/reconfigured connection. These paths
are still a development preview and require physical-device lifecycle tests.

## Build

Use Java 17, Gradle 8.9 and Android SDK 35:

```sh
gradle --no-daemon :app:assembleDebug :app:lintDebug
```

Run this from `android/`. The Android GitHub workflow installs the tools, compiles
the APK and runs lint. Release APKs are development/debug-signed builds, not
Play Store packages. A rebuild may use a different debug certificate, requiring
uninstall/reinstall; that clears the locally saved gateway credential.

## Scope of this preview

- Real SMS through the default or explicitly selected SIM; multipart submission results.
- Persistent pending-result state, no automatic carrier resend after restart.
- Foreground operation started by the user; no boot-start receiver.
- No inbox-reading permission, broad contact access or background SMS scraping.
- No QR enrolment, carrier delivery reports or
  production app-store distribution yet.

The APK is build/lint checked. Physical device behavior, OEM permission policies,
carrier restrictions and native OTP autofill have not been validated in CI.
HTTP is supported for a trusted local network; use HTTPS for other connections.
