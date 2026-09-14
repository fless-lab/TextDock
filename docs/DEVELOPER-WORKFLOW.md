# Daily development workflow

## Projects and inboxes

Use the inbox selector and folder action in the header to create a project or
an additional inbox. A project gets one default inbox. Capture into a selected
inbox by including `inbox` in the JSON request; omitted means `local` for backwards
compatibility. Twilio capture accepts `X-TextDock-Inbox`, otherwise `local`.

Messages, searches, OTP retrieval and paired phone sessions carry an inbox ID.
Phone pairing is bound to the inbox selected on the desktop. The shared admin
token can manage all projects; local projects are organization tools, not SaaS
tenant identities or role-based access controls.

## Browse and inspect

- Search body/recipient text, then refine by recipient, test run, exact tag or
  inclusive creation time. Favorites and detected-OTP views filter on the server.
- Pages contain at most 100 messages in the UI; API clients can request 1–200.
  `next_cursor` uses creation time and ID, so new arrivals do not duplicate older
  rows across pages. Change a filter to start again at page one.
- Open a message to star it, edit up to ten tags or view its recipient's
  conversation. The conversation action filters that inbox's message list to
  the selected recipient; pagination remains available.
- Select individual rows or the current page to delete a bounded batch.
- **Export page** downloads a CSV of the current filtered page. Per-message JSON
  export remains available. CLI export traverses every page with bounded memory.
- `/` focuses search. `Ctrl+K` / `Cmd+K` opens common commands.
- Unicode-triggering characters appear in the encoding details.

## CLI

All clients use `TEXTDOCK_URL` (default `http://127.0.0.1:18257`) and optionally
`TEXTDOCK_TOKEN`. These commands operate on the running server:

```sh
textdock send --to +33612345678 --from Acme --body 'Your code is 482193' --run-id signup-42
textdock list --inbox local --run-id signup-42 --limit 20
textdock wait --to +33612345678 --run-id signup-42 --timeout 30
textdock export --inbox local --format jsonl > messages.jsonl
textdock export --inbox local --format csv > messages.csv
textdock purge --inbox local
```

`list` supports `--cursor`, `--q`, `--to`, `--run-id`, `--tag`, `--favorite` and
`--otp`. `export` supports the same filters and `json`, `jsonl`, `csv`. CSV exports
prefix formula-like cells with an apostrophe for spreadsheet use; JSON preserves
the original text. Export is a traversal, not an immutable point-in-time snapshot;
concurrent deletion/retention can remove rows before later pages are read.

## Configuration and retention

```sh
textdock --config examples/textdock.json
TEXTDOCK_RETENTION=24h textdock
textdock --otp-pattern 'code=([A-Z0-9]{6})'
```

Precedence: CLI flags → environment → JSON config → defaults. Config accepts
`listen`, `db`, `public_url`, `retention`, `otp_pattern`. `TEXTDOCK_CONFIG` selects a
file; the admin token is environment-only. Unknown config keys fail startup.
Relative database paths are relative to the process working directory.

Retention is disabled by default. A positive Go duration (`24h`, `168h`) deletes
messages older than that duration at startup and once per minute. This applies
to all inboxes, including favorites; paired views resync after cleanup.

The custom OTP pattern uses RE2 syntax. The first capture group becomes the
candidate code (up to 64 bytes). This replaces the default numeric heuristic;
the application remains responsible for validating and expiring codes.

## Backup and restore

```sh
textdock backup --db data/textdock.db --out snapshot.db
textdock restore --source snapshot.db --db restored/textdock.db
textdock --db restored/textdock.db
```

Backup uses SQLite `VACUUM INTO` for a consistent standalone snapshot, including
messages, projects and device credential hashes. It can coexist with a running
server of the same version. Use the currently installed version to back up
before upgrading. Backup opens the database with that version's migrations.

Restore checks SQLite integrity and supported schema, then copies to a **new**
destination. It never overwrites an existing database. Stop the server and select
the restored file with `--db`; do not replace an open database or copy its main
file without accounting for the WAL. Protect snapshots as you protect the inbox.

## Automated OTP tests

Use a fresh `run_id` for every execution and pass it through your application's
SMS adapter. The dependency-free Node helper is in `examples/testing/textdock.mjs`.

Playwright:

```js
import { randomUUID } from 'node:crypto';
import { waitForCode } from './textdock.mjs';

const runId = randomUUID();
// Configure your test application's SMS adapter with this run ID, then trigger login.
const code = await waitForCode({ to: '+33612345678', runId, token: process.env.TEXTDOCK_TOKEN });
await page.getByLabel('Verification code').fill(code);
```

Cypress (use `cy.request`, which runs outside the browser's CORS boundary):

```js
cy.request({
  url: 'http://127.0.0.1:18257/api/v1/otp',
  qs: { inbox: 'local', to: '+33612345678', run_id: runId, timeout: 30 },
  headers: { Authorization: `Bearer ${Cypress.env('TEXTDOCK_TOKEN')}` },
  timeout: 35000,
}).then(({ body }) => cy.get('input[autocomplete="one-time-code"]').type(body.code));
```

Any backend test runner can use the same HTTP endpoint or CLI `wait`. These tests
verify the application's OTP flow, not the phone platform's native SMS autofill.
