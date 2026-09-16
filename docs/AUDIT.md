# Audit history

Open **Manage projects → Audit log**, or use **Settings → Audit log**. Operators
can inspect all project and instance events. A project administrator can inspect
only projects where they currently have the `admin` role. Viewers, members and
machine API keys cannot read the journal, even with message read/delete access.

The UI filters by project, action, outcome and actor ID. **Older events** uses
keyset pagination; **Newest events** refreshes the head of the journal.
**Download audit page** exports only the currently loaded, authorized metadata.

## Recorded metadata

Events have a sequential ID, UTC timestamp, actor kind/ID, action, outcome,
project/inbox scope, resource type/ID and affected-resource count. Membership
events include previous/new roles; API-key creation includes its declared
permissions. Actor and resource IDs identify stored records, not credentials.

The vocabulary is deliberately closed. Audit rows contain **no SMS body, OTP,
phone number, run ID, tag value, display name, username, password, credential
hash, token, request/response body, arbitrary URL or free-form error details**.

## Actor attribution

| Actor kind | Meaning |
|---|---|
| `user` | Authenticated user ID, including session creation after password verification |
| `api_key` | Authenticated machine-key ID; never its secret or hash |
| `operator` | A request using the shared server token |
| `local_operator` | Anonymous operator access in the existing loopback-only mode |
| `system` | Storage operations without a request identity, such as message retention |
| `anonymous` | Failed password checks; the submitted username is not a verified identity |

A shared operator token cannot identify an individual human. Use named accounts
for individual attribution. Headers or body fields supplied by clients cannot
choose the recorded actor. API requests made by the CLI/SDK use the same
credential-derived attribution as other clients.

## Covered actions

| Actions | Scope and meaning |
|---|---|
| `project.created`, `inbox.created` | Committed project/inbox creation, including a new project's initial inbox |
| `message.created` | A persisted message record, including capture, simulation or relay/lab intent |
| `message.updated`, `message.deleted` | Metadata update or individual deletion; batch deletion produces per-message events |
| `messages.purged`, `messages.pruned` | Aggregate deletion count per inbox for explicit purge or message retention |
| `scenario.saved`, `scenario.deleted` | Scenario configuration mutation, without callback URLs or configuration contents |
| `api_key.created`, `api_key.revoked` | Scoped machine-key lifecycle |
| `user.created`, `user.enabled`, `user.disabled`, `user.password_changed` | Instance-level account lifecycle |
| `session.created`, `session.revoked` | Instance-level session creation or explicit revocation |
| `membership.assigned`, `membership.removed` | Project-level role changes; associated session revocations belong to this transaction |
| `login.failed` | An eligible password attempt that reached the verifier and was denied |
| `export.prepared` | Number of message records prepared for JSON/CSV export |

The journal starts after upgrading to schema 11. Existing data is not presented as
newly created history. Repeated deletion of a missing message and repeated
revocation of an already revoked key/session do not create duplicate events.
Saving a membership can still revoke sessions even when its role value is unchanged.

## Outcomes and durability

- **`committed`**: the event and resource mutation committed in the **same SQLite
  transaction**. If audit insertion fails, that covered mutation rolls back,
  including any associated session revocation. It is not a record of a request
  that was merely attempted.
- **`denied`**: a password check failed. This is appended separately, within the
  existing login budgets. Malformed inputs and rate-limited attempts that never
  reach password verification are not recorded.
- **`prepared`**: the server selected records for an export and persisted the
  audit event before returning the data. It does not prove the client received,
  opened or saved the download.

`message.created` is not a carrier receipt, callback outcome or native-device
delivery confirmation. Those remain in the existing message/relay/lab histories.
This preview is not an HTTP access log: routine reads, OTP polling, audit reads,
all authorization rejections, phone/push/gateway enrollment and background
delivery transitions are not comprehensively journaled here.

## API

```sh
curl -H "Authorization: Bearer $TEXTDOCK_TOKEN" \
  'http://localhost:18257/api/v1/audit?project_id=default&action=message.deleted&limit=50'
```

Use an operator or project-admin **user session** credential. Project admins must
provide an explicit `project_id`; an `inbox_id` filter must belong to that project.
Operators may omit the project for a global view or filter directly by an inbox.

Supported filters are `project_id`, `inbox_id`, `action`, `outcome`, `actor_id`,
`since` (RFC3339), `before` (positive event ID) and `limit` (1–200). Results are
ordered by descending event ID. `next_cursor` is the next `before` value; zero
means that page has no older results. New events do not shift already paginated
IDs. Cursors belong to the current database history; refresh after restoring a
snapshot.

See [audit OpenAPI](../api/audit.openapi.yaml) for the event schema and filters.

## Storage and retention

SQLite schema 11 adds `audit_events` and project/inbox indexes. Audit rows have no
cascading foreign keys: deleting or retaining SMS does not erase their audit
history. There is no audit update/delete API or automatic audit-retention policy
in this preview. Backups preserve the journal; a restore replaces it with the
snapshot's history. Plan storage capacity with this retained metadata in mind.

This is a local database journal, **not a signed or tamper-evident ledger**.
Filesystem/database administrators can change it. External archival, audit
retention controls and broader worker/device coverage are later milestones.
