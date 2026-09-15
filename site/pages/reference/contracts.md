# OpenAPI contracts

TextDock's APIs are described in OpenAPI 3.1 documents. Download the related
files together: some contracts reference schemas in `openapi.yaml`.

| Contract | Scope | Download |
|---|---|---|
| Capture API | Messages, search, metadata and OTP waits | [openapi.yaml](/api/openapi.yaml) |
| API keys | Operator-managed scoped machine credentials | [keys.openapi.yaml](/api/keys.openapi.yaml) |
| User accounts | Password sessions and project memberships | [users.openapi.yaml](/api/users.openapi.yaml) |
| Phone connection | Pairing, scoped sessions and live invalidations | [connect.openapi.yaml](/api/connect.openapi.yaml) |
| Phone notifications | Web Push subscription and diagnostics | [push.openapi.yaml](/api/push.openapi.yaml) |
| Workspaces | Projects, inboxes, bulk deletion and export | [workspace.openapi.yaml](/api/workspace.openapi.yaml) |
| Simulation | Scenarios, events, inbound SMS and callbacks | [simulation.openapi.yaml](/api/simulation.openapi.yaml) |
| Provider subsets | Vonage and OVH capture shapes | [providers.openapi.yaml](/api/providers.openapi.yaml) |

The desktop API is protected by the server token when configured. Paired phone
credentials authorize only their scoped read-only routes. Provider capture
adapters use the local-token conventions in the [compatibility guide](/guide/providers).

## Typical request

```sh
curl http://localhost:18257/api/v1/messages \
  -H 'Content-Type: application/json' \
  -d '{"to":"+12025550123","from":"Acme","body":"Code 482193"}'
```

Add `Authorization: Bearer YOUR_TEXTDOCK_TOKEN` when the server token is enabled.
See the [HTTP reference](/reference/api) for filters, limits and error behavior.
