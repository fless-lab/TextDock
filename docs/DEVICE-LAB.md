# Android device lab

The device lab injects simulated incoming SMS into an explicitly selected Android
emulator. It uses ADB on the TextDock server. It does not use the physical-phone
relay, a SIM subscription or provider credentials.

## Enable and discover

Install Android Platform Tools, start an Android Virtual Device, then run:

```sh
TEXTDOCK_ADB_ENABLED=true textdock
```

If `adb` is not on the server's PATH, set `TEXTDOCK_ADB_PATH` to its executable.
`TEXTDOCK_ADB_TIMEOUT` controls each command's timeout (default 8s, range 1s–30s).
The overall injection is bounded to 30s plus a short result-persistence window.

Open **Device lab** in TextDock. It displays the ADB version and discovered
devices, including offline/unauthorized devices. Choose the target explicitly:
no device is automatically selected. Injection is limited to online
`emulator-NNNN` targets whose boot and virtual SIM are ready. Physical devices
remain visible but cannot be selected for this operation.

ADB is optional and disabled by default. It is not bundled into the standalone
binary or Docker image. Enabling discovery can start the normal local ADB daemon.
The native binary is the simplest setup; containers need an operator-configured
ADB executable/server connection. Never expose an unauthenticated ADB server.

## Send a simulated SMS

Choose the emulator, a numeric sender and an inbox test recipient. The **serial**
chooses where the message is injected. The test recipient only groups the record
inside TextDock; it does not change the emulator's phone number.

```sh
textdock devices
textdock inject --serial emulator-5554 \
  --from +12025550100 --to +12025550123 \
  --body 'Your code is 482193' --run-id emulator-login-42 \
  --idempotency-key emulator-login-42
```

API:

```json
{
  "serial": "emulator-5554",
  "inbox": "local",
  "from": "+12025550100",
  "to": "+12025550123",
  "body": "Your code is 482193.\n\n@login.example.test #482193",
  "run_id": "emulator-login-42",
  "idempotency_key": "emulator-login-42"
}
```

POST this to `/api/v1/lab/injections` with the usual desktop authorization.
Request bodies are limited to 1024 UTF-8 bytes of SMS text. CRLF is normalized to
LF; unsupported control characters are rejected. The sender must be an
E.164-shaped number, not an alphanumeric sender ID.

TextDock starts ADB directly, without a host shell. Message text uses the console's
documented Unicode escapes, with explicit UTF-16 surrogate pairs for supplementary
characters. Line breaks and backslashes are escaped; no raw command delimiters
reach the console. Passing raw non-BMP UTF-8 to `sms send` can alter characters on
some versions. The legacy `sms pdu` command is not used because some newer modem
implementations acknowledge it without delivering to the guest.

## Results and history

Every attempted injection has a persistent message record and lab history:

- `injecting`: the command is in progress;
- `injected`: the console accepted all parts;
- `failed`: the console explicitly rejected the first part;
- `unknown`: interruption, incomplete multipart submission or an ambiguous result.

These are emulator outcomes, not carrier delivery receipts. Interrupted attempts
are not automatically repeated. A stable idempotency key returns the existing
attempt even if the emulator is no longer available. Changed request data or a
deleted original produces a conflict. Key fingerprints survive message deletion
so a stale retry cannot create another SMS.

The inbox provides the original text, OTP analysis and events. The lab view lists
the latest attempts for the selected inbox; `/api/v1/lab/history` supports a
bounded `limit` up to 200. CLI injection prints the result and exits nonzero if
the outcome is not `injected`.

## Testing boundary

The browser suite uses a test-only ADB executable. A separate GitHub workflow
boots a disposable Android emulator and checks its SMS inbox for the actual
received content, including quotes, backslashes, line breaks and Unicode.
Product code does not root devices or read their SMS inboxes; root inspection in
that workflow applies only to its disposable emulator fixture.

This does not prove SMS Retriever, WebOTP or iOS autofill on physical hardware.
Use the correct app hash/origin and keep manual-entry/cancellation paths in the
application under test. Physical relay validation remains documented in
[RELAY.md](RELAY.md).

## Remaining device-lab work

USB modem support, broader device/OEM matrices and background Web Push remain
future work. A successful emulator test is not a claim that those features are
implemented or that carrier delivery has been verified.
