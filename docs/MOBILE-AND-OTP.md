# Mobile inbox, native SMS and autofill

## Capability map

| Path | Real SMS in native inbox? | Native autofill? | Requirements | Status |
|---|---|---|---|---|
| TextDock phone browser | No | No; select/copy code | Same LAN, server token | v0.1 |
| OTP API + test runner | No | Test fills application field | A run ID and test integration | v0.1 |
| Scoped QR phone session | No | No | Pairing flow | v0.2 |
| Android Emulator SMS injection | Simulated in emulator | Depends on image/API | Android SDK, ADB, emulator | v0.7 preview |
| Provider relay | Real-send implementation; device validation pending | If receiver/app/browser supports it | Provider credentials, Internet, cost | v0.6 beta |
| Android SIM gateway | Real-send implementation; device validation pending | If supported | Gateway Android, app, SIM/plan | v0.6 beta |
| USB cellular modem | Yes, on destination phone | If supported | Compatible modem, SIM/plan | Planned v0.7 |
| PWA push notification | No | No | HTTPS, permission, push infrastructure | Planned v0.7 |

A captured message cannot be written by a website into iOS or Android's native
SMS inbox. Reading a message in TextDock does not trigger OS SMS detection.
Filling a field with a test runner verifies application flow, not telephony.

Use the [device lab](DEVICE-LAB.md) to select a running Android emulator and inject
a simulated SMS through its console. This is distinct from the physical SIM gateway.

The current relay preview is described in [RELAY.md](RELAY.md), with a
[development Android companion](../android/README.md). The APK builds and protocol
tests pass, but physical receipt/autofill has not yet been verified.

## Same-Wi-Fi inbox today

```sh
TEXTDOCK_TOKEN='your-long-local-token' ./textdock --listen 0.0.0.0:18257
```

Use **Open on phone** in the desktop interface to pair through a QR code. The
phone gets a read-only credential restricted to the chosen recipient/run.
See [CONNECT.md](CONNECT.md). `localhost` on a phone means the phone itself.
Opening the root desktop interface directly and entering its server token is
still possible, but grants full shared access rather than a restricted session.

The computer's firewall must allow that port and Wi-Fi client isolation must
be disabled for these devices. HTTP on an untrusted network can expose the token
and messages; use a trusted LAN or an HTTPS reverse proxy. Clipboard API support
usually requires a secure context; text/code remains selectable if copy fails.
Live events update the open view; delivery while suspended or screen-locked is
not guaranteed. A reconnect refreshes the scoped message list.

For Docker LAN use, change the Compose mapping to `18257:18257` and set your own
token. The shipped Compose file intentionally binds the host port to loopback.

## Web application: real SMS + WebOTP

The **application under test**, not the TextDock inbox, implements WebOTP. On a
compatible Android browser it runs on an HTTPS origin and begins listening
before the SMS arrives. A common SMS format is:

```text
Your Acme code is 482193.

@login.example.test #482193
```

The final line binds the code to the actual application's hostname, with no
scheme, path or port. The phone must resolve and trust the application's HTTPS
hostname; a local computer's HTTP IP address is not a drop-in substitute.
Embedded cross-origin flows require their own origin format and permissions.

```html
<input name="code" inputmode="numeric" autocomplete="one-time-code" />
```

This annotation permits supported platforms to suggest received OTPs. On
compatible Chromium browsers the application can also feature-detect
`OTPCredential` and call `navigator.credentials.get` with an SMS transport and an
abort timeout. Consent/platform policies still apply. Safari's OTP suggestion
is not equivalent to support for the WebOTP JavaScript API. Always retain manual
entry and cancellation paths.

The `examples/webotp/` page demonstrates integration only; it neither sends a
real SMS nor bypasses the platform. Serve it on your trusted HTTPS test origin.

## Native Android application

- **SMS Retriever API**: app starts listening through Google Play services;
  received text includes the correct app hash derived from package/signing
  certificate, follows the Retriever size/format requirements, and arrives in
  the listening window. Requires no broad SMS inbox-reading permission.
- **SMS User Consent API**: platform asks the user to permit access to one
  qualifying incoming message. Sender/code eligibility restrictions apply.
- **Emulator**: the Android console can inject a received SMS:

  ```sh
  adb -s emulator-5554 emu sms send +15551234567 "Your code is 482193"
  ```

  This can exercise emulator receipt and application UI without a provider. It
  does not prove carrier delivery or identical Retriever behavior on every
  Google Play services/emulator image. The future connector should select a
  device explicitly, not broadcast to all attached devices.

## Native iOS application

Use `.textContentType(.oneTimeCode)` in SwiftUI or `textContentType = .oneTimeCode`
on a UIKit field. Supported iOS versions can offer the code after a qualifying
real SMS arrives. The user may need to choose the suggestion.

There is no supported public mechanism for TextDock to silently insert a fake
SMS into a physical iPhone's Messages app. Simulator/test-runner field filling
must be labeled as application testing, not validation of real SMS autofill.

## Real SMS without an API provider

An Android gateway or cellular modem sends through **its SIM and mobile
operator**. A separate destination phone receives the actual SMS. This removes
the SMS API vendor, not the carrier or possible plan charges. Sending to the
gateway's own number is not a portable test setup and must not be assumed.

The Android gateway will be an installed native app. It needs SMS sending
permissions, must account for Android background execution rules and app-store
distribution restrictions, and should report submission/delivery separately.
An iPhone is not a drop-in silent SMS-sending gateway. Never equate acceptance
by the gateway with arrival on the receiving phone.

## Sources

- [Chrome WebOTP guide](https://developer.chrome.com/docs/identity/web-apis/web-otp)
- [SMS OTP form practices](https://web.dev/articles/sms-otp-form)
- [Android SMS Retriever](https://developers.google.com/identity/sms-retriever/overview)
- [Android SMS User Consent](https://developers.google.com/identity/sms-retriever/user-consent/overview)
- [Android emulator console](https://developer.android.com/studio/run/emulator-console)
- [Apple oneTimeCode](https://developer.apple.com/documentation/uikit/uitextcontenttype/onetimecode)
