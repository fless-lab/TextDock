# Contributing

TextDock welcomes small, understandable changes with a complete user journey.
Discuss larger additions against the versioned roadmap before expanding scope.

## Development

Requirements: Go 1.26+, Node.js 24+, npm, Make. A C compiler is needed for Go's
race detector, not for the distributed pure-Go SQLite binary.

```sh
make setup
make build
./bin/textdock
```

For UI hot reload, run the backend above and `npm --prefix web run dev` in a
second terminal. Vite proxies `/api` to `127.0.0.1:18257`; adjust the development
proxy if you change the backend port. Production serves UI/API on one port.

```sh
npm --prefix web run format
go fmt ./...
make check test
npm --prefix web exec -- playwright install chromium
make e2e
node scripts/smoke.mjs
```

The first browser install may need Linux system dependencies (`playwright
install --with-deps chromium`). E2E starts its own authenticated, in-memory
server on **18258**, separate from the default development instance.
The executable smoke test checks the default port **18257**; stop your own
TextDock instance before running it.

## Design expectations

- Keep domain logic independent of transports and storage.
- Use bounded responses, cancellation and parameterized SQL.
- Add a persistence migration for every schema change after v0.1.
- Document exact compatibility and reject unsupported provider behavior.
- Preserve the difference between capture, simulation and real delivery.
- UI features need loading, empty, error, keyboard and phone states.
- Test behavioral boundaries, not copies of implementation details.
- Do not store real SMS, tokens, databases or generated binaries in Git.

Use focused Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`). Add
user-facing changes under `Unreleased` in the changelog. Maintainers cut releases
using [RELEASING.md](docs/RELEASING.md); contributors should not move release tags.
