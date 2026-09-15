# Native OTP receiving samples

Two small receiving applications demonstrate OTP input:

| Sample | Integration | Build |
|---|---|---|
| Android 8+ | Google SMS Retriever and SMS User Consent, plus manual entry | Development APK in the release |
| iOS 16+ | SwiftUI `.textContentType(.oneTimeCode)`, plus manual entry | Simulator app in the release; Xcode source for physical devices |

Both accept **six ASCII digits** and show a local confirmation. They do not send
SMS, call a verifier or establish an authenticated session. In your application,
submit the code to your backend and enforce its expiry, attempt limits and
single-use semantics there. The sample's five-minute window is only local UI
state; it does not make a code valid at a server.

## Android: install and receive

Download `textdock-otp-sample-vVERSION.apk` from the chosen release. It is a
separate application from the **TextDock Gateway** sending app.

```sh
adb -s emulator-5554 install textdock-otp-sample-vVERSION.apk
adb -s emulator-5554 shell am start -n dev.textdock.otpsample/.MainActivity
```

Choose the actual serial explicitly. For a physical device, install through your
normal development workflow. Debug APKs from different builds can have different
signing certificates; uninstall the old sample first if Android rejects an update.

### SMS Retriever

1. Open **TextDock OTP Sample** on the receiving device.
2. Copy the **installed signing hash**, or press **Copy Retriever test SMS**.
3. Press **Start SMS Retriever** and wait for **Listening**.
4. Deliver this message, replacing the final line with the installed app's hash:

   ```text
   <#> Your verification code is 482193
   YOURAPPHASH
   ```

5. If Google Play services retrieves the SMS, the sample fills an empty field.
   Review it and press **Use code (demo)** for a local confirmation.

The displayed hash derives from this installed APK's package name and signing
certificate. A rebuild or a Play-distributed signing key can change it. Do not
hard-code a hash copied from another build. Retriever messages must stay within
the platform's 140-byte limit. The short copied message satisfies that limit.

TextDock's **Relay → Native OTP message format** helper can generate the message
using that hash. For a simulated Android device, paste it into **Device lab** and
inject it into the explicitly selected emulator. For a physical receiving phone,
use a configured [real relay](RELAY.md) addressed to its real number.

Retriever needs compatible Google Play services and an eligible incoming SMS.
Native Messages receipt on an emulator does not guarantee Retriever will emit a
result on that image. The app reports startup failures and timeout and retains
manual entry.

### SMS User Consent

1. Press **Start User Consent** and wait for **Listening**.
2. Deliver `Your verification code is 482193` to the receiving device.
3. If Google Play services recognizes an eligible message, choose whether to share
   **that one message** in the system dialog.
4. Approval fills an empty field; rejection leaves manual entry available.

No Retriever hash is required. This example passes no sender filter; a real
application can provide a known sender to `startSmsUserConsent`. Google applies
message/sender eligibility rules, including a 4–10 character alphanumeric code
containing a number. The sample deliberately parses only a single six-digit ASCII
candidate and rejects ambiguous messages with multiple candidates.

The app requests no SMS-reading or SMS-sending permission. Its dynamic receiver
requires the Google Play services signature permission, with the explicit
exported-receiver flag required on recent Android versions.

### Cancellation and activity lifecycle

**Cancel listening** removes the app's receiver and rejects late callbacks.
Listening also expires after five minutes. Leaving the activity stops listening,
except while Google's one-message consent dialog covers it. Rotation/recreation
does not silently restart a listener or persist the code; start again or type it.
Incoming text never overwrites a code already being edited. SMS bodies and codes
are not logged or saved by the sample.

## iOS: run and receive

For simulator form testing, unzip `textdock-otp-ios-simulator-vVERSION.zip` on
macOS and choose an available iPhone simulator UDID:

```sh
xcrun simctl list devices available
xcrun simctl boot YOUR_SIMULATOR_UDID
xcrun simctl install YOUR_SIMULATOR_UDID TextDockOTP.app
xcrun simctl launch YOUR_SIMULATOR_UDID dev.textdock.otpsample
```

The archive targets the simulator architecture used by the release runner. Build
from source for a different simulator architecture or a physical iPhone.

For a physical-device check, build the source with Xcode, select your own signing
team and install on your receiving iPhone. Focus **Verification code**, send a real
SMS such as `Your verification code is 482193`, and choose the code suggestion if
iOS offers one. The annotation exposes the platform input hint; it does not grant
inbox-reading access. No associated-domain entitlement is preconfigured; an
application using domain-bound codes must configure its own domains and SMS format.

**New attempt** opens a five-minute local entry window. **Cancel entry** and expiry
clear the field; a fresh attempt restores manual entry. Returning from the
background rechecks the deadline. **Use code (demo)** captures input locally and
clears it. No code is persisted between app launches.

The simulator does not receive real SMS. Typing a code with XCTest tests form
behavior, not SMS receipt or automatic keyboard suggestions.

## Build from source

Android, from `android/`, with Java 17, Gradle 8.9 and Android SDK 35:

```sh
gradle --no-daemon :otp-sample:assembleDebug :otp-sample:lintDebug :otp-sample:testDebugUnitTest
# With one explicitly selected test emulator available:
ANDROID_SERIAL=emulator-5554 gradle --no-daemon :otp-sample:connectedDebugAndroidTest
```

The receiving sample depends on `play-services-auth-api-phone`; the SIM gateway
still uses only Android platform APIs.

iOS, from `examples/ios-otp/`, on macOS with Xcode and XcodeGen:

```sh
brew install xcodegen
xcodegen generate
open TextDockOTP.xcodeproj
# Or run automated form/unit tests on an existing simulator:
xcodebuild -project TextDockOTP.xcodeproj -scheme TextDockOTP \
  -destination 'platform=iOS Simulator,id=YOUR_SIMULATOR_UDID' \
  -derivedDataPath build CODE_SIGNING_ALLOWED=NO test
```

The Xcode project is generated from `project.yml`. For physical-device builds,
choose a development team in Xcode and use normal code signing.

## Validation matrix

| Check | Automated | What still needs a physical-device check |
|---|---|---|
| Android code parser/hash/window | Unit tests: exact code policy, signing-hash vector, cancellation and expiry | Signing/distribution variants |
| Android sample form | Emulator UI tests: manual entry, invalid input, cancellation and recreation | Real Google consent and Retriever receipt, OEM/background behavior |
| iOS code policy/window | Unit tests: ASCII policy, cancellation, expiry and cleared input | System suggestion timing |
| iOS sample form | Simulator UI test: entry, cancellation and a fresh attempt | SMS keyboard suggestion on a receiving iPhone |
| Emulator SMS transport | Separate TextDock lab integration checks native virtual inbox contents | Carrier delivery and physical SIM behavior |

For physical checks, record app version/signing hash, OS/device, Google Play
services version where applicable, sending path and whether the code was typed,
suggested or supplied after consent. Include refusal, late receipt, cancellation
and manual-entry cases. Keep actual codes and message bodies out of shared reports.

## Platform references

- [SMS Retriever](https://developers.google.com/identity/sms-retriever/overview)
- [SMS User Consent](https://developers.google.com/identity/sms-retriever/user-consent/request)
- [Apple oneTimeCode](https://developer.apple.com/documentation/uikit/uitextcontenttype/onetimecode)
