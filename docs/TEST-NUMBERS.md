# Phone numbers and OTPs in tests

## Does TextDock generate phone numbers?

**No. TextDock currently captures messages addressed to a number you choose.**
It does not allocate numbers, register a SIM, rent a provider number or prove
that a number exists. A future fixture generator could produce repeatable test
data, but that would still not create a reachable mobile number.

## Why use a number at all?

The recipient is an address inside the development environment. It lets you:

- associate messages with a test user;
- search and group a conversation;
- restrict a paired phone to one recipient;
- match a simulation scenario's prefix;
- find an OTP for the correct recipient and unique test run.

The same number can appear in different inboxes. Pairing includes the inbox and
optionally a run ID, so tests in different projects stay separate.

## What is validated?

The `to` field must have an international-number shape: a plus sign followed by
7–15 digits, starting with a nonzero digit. This is a syntax check, not carrier
validation or a complete country numbering-plan validator.

`from` is the sender label or number, such as `Acme`. Local validation allows a
nonblank sender up to 64 characters; real providers impose their own sender
approval, length and destination rules.

For a synthetic example, you can use `+12025550123`, in the North American
555-01xx fictional-use range. Keep test fixtures separate from real customer
data and give every test execution a new `run_id`.

## Does phone pairing verify ownership of the number?

No. Pairing authorizes that browser to read a development inbox scope. You can
pair a phone to a synthetic recipient that has no relationship to its SIM.
This is useful for testing, but it is not phone-number verification.

## What changes with real SMS?

To receive a real SMS in the native Messages app, use the actual destination
phone's reachable number. Sending requires an authorized provider or a
SIM-equipped gateway. A generated fixture or a local inbox address cannot make
a carrier route an SMS to your physical phone.

Obtaining a real sender number is a separate action performed through a mobile
operator or a provider. TextDock does not provision one automatically.

## Who generates the OTP?

Your application generates, expires and verifies the code. TextDock captures the
message and extracts a candidate to copy or retrieve in tests. The composer has
an example code; it is not a secure OTP issuance service.

The default extractor looks for standalone 4–8 digit candidates. You can set a
custom RE2 pattern. Use recipient, inbox and a fresh run ID when waiting for a
code, and optionally require `status=delivered` in a simulation test.
