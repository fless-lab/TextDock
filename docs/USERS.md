# Users, sessions and project roles

The second v0.8 preview adds local user accounts to the self-hosted desktop UI.
The server operator creates accounts; project memberships control which inboxes
each user can see and modify. No hosted account or email service is required.

## Start and create the first account

1. Configure `TEXTDOCK_TOKEN` on the server and start TextDock. Network binding
   retains the existing minimum 16-character operator-token requirement.
2. Open the desktop and unlock it with that operator token.
3. Open **Manage projects → Manage team** (also available from Settings).
4. Create an account with a username, display name and initial password.
5. Select a project and assign the account a role using its username.
6. Give the teammate the server URL and their initial credentials. On the login
   page they choose **Sign in with a user account**.

Usernames are normalized to lowercase, 3–64 characters, using letters, digits,
dot, dash and underscore. Passwords are 12–128 characters. A new account has no
project access until assigned a membership. Account creation and membership
assignment are separate operations.

The operator token remains the installation-wide administration/recovery
credential. A **project admin is not a server operator**. Account/password
administration and machine API-key management remain operator-only.

Anonymous loopback startup is still available for single-user local development.
It does not enforce user isolation: anyone reaching that anonymous API has
operator access. Account administration and password login require the server
token to be configured. Use the existing trusted HTTPS setup for remote sign-in.

## Role matrix

| Operation in an assigned project | Viewer | Member | Project admin |
|---|---|---|---|
| Read messages, OTPs, events and exports | Yes | Yes | Yes |
| Capture local SMS and edit tags/favorites | No | Yes | Yes |
| Delete messages or purge an inbox | No | No | Yes |
| Create an inbox in that project | No | No | Yes |
| View project members | Yes | Yes | Yes |
| Assign/change/remove project memberships | No | No | Yes |
| Create projects or accounts, reset other passwords | No | No | No |
| Manage machine keys, gateways, phones or emulator targets | No | No | No |
| Start real relay, simulation or webhook replay | No | No | No |

Users can have different roles in different projects. The role for the requested
resource is used for authorization; a writable role in one project never makes
another project's viewer role writable. Inbox selection must be explicit in API
requests. The UI automatically selects an accessible inbox.

For batched deletion, all message IDs must belong to the same authorized project.
Missing/out-of-scope IDs reject the request before deletion begins. New inboxes
inherit the user's existing project role. No inbox-specific user roles are
provided in this preview; [machine API keys](API-KEYS.md) can be inbox-specific.

## Sessions and account controls

Each successful login creates a random `td_user_…` Bearer credential with a
**12-hour absolute expiry**. It is stored as a hash in SQLite and in the browser's
tab-scoped `sessionStorage`; it is not an authentication cookie. Reopening the
same tab can reuse the session until expiry. Duplicating a tab can copy its token.
Server restarts preserve active sessions in the database.

**Your account** shows active sessions and permits:

- signing out of the current session;
- ending another session owned by the same user;
- changing the password after supplying the current password.

Password changes, operator password resets, account disable/enable and membership
updates revoke **all sessions for that user**, including sessions viewing another
project. The user signs in again to receive their current roles. Removing the last
project admin is permitted because the server operator retains recovery access.

The desktop clears its view and credential when it receives revocation or an
authentication failure. Old asynchronous capture/read responses are ignored after
an account or inbox switch. Active inbox streams are scoped; an account without
memberships can still subscribe to its own session-lifecycle events.

Revocation blocks subsequent authorization checks and ends live streams. Requests
already admitted may finish, and revocation cannot erase content already viewed,
copied or downloaded. A disconnected browser learns about revocation on reconnect.

## Password handling and login limits

- Passwords use Argon2id with independent random 16-byte salts, 64 MiB memory,
  two iterations, one lane and a 32-byte result. Password hashes are never returned
  by account APIs.
- Session tokens use 256 random bits and are stored as SHA-256 hashes.
- Login failures use a generic username/password error; eligible unknown-account
  attempts still perform password-hash work.
- Fixed one-minute budgets permit 10 attempts per normalized account and 60
  total login attempts per process. Self-service password changes share the
  account budget. Up to four password operations run concurrently.
- Limits return HTTP 429 with `Retry-After`; the in-memory budgets reset on restart.

The browser token remains readable by same-origin application JavaScript. The
existing CSP, cross-origin checks and HTTPS deployment boundary apply. Passwords
and session secrets are not included in application logs or account listings.

## API usage

The [user/team OpenAPI contract](../api/users.openapi.yaml) describes account,
membership, login and session operations. For example:

```http
POST /api/v1/auth/login
Content-Type: application/json

{"username":"alice","password":"your-account-passphrase"}
```

The response contains a one-time `token` and `session` metadata. Send the token in
`Authorization: Bearer td_user_…` for supported desktop API routes.

`GET /api/v1/account` reads your session; `POST /api/v1/account/logout` revokes it.
`GET /api/v1/account/sessions` lists only your active sessions.
`DELETE /api/v1/account/sessions/{id}` cannot revoke another user's session.
`PUT /api/v1/account/password` takes `current_password` and `password`.

Use **machine API keys** for CI and provider-shaped capture adapters. A user
session cannot authorize a Twilio/Vonage/OVH adapter or become a phone/gateway
credential. The operator UI retains all existing integration controls.

## Storage and remaining team work

SQLite schema 10 adds users, memberships and sessions. Database backups include
password hashes, grants and session revocation state. Restoring a snapshot can
restore earlier passwords, memberships and sessions; review them after recovery.

This release does not implement email invitations, self-registration, OIDC/MFA,
audit history, PostgreSQL, hosted organizations or billing. Physical SMS/autofill
validation remains separate from user authorization tests.
