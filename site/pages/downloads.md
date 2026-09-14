# Downloads

**TextDock {{VERSION}}** · [Release notes]({{RELEASE_URL}}) · [All releases](https://github.com/fless-lab/TextDock/releases)

The standalone binary includes the web interface. Node.js and an external
database are not needed to run TextDock.

## Standalone binaries

| Platform | Architecture | Download |
|---|---|---|
| Linux | x86-64 / amd64 | [tar.gz]({{ASSET_URL}}/textdock_v{{VERSION}}_linux_amd64.tar.gz) |
| Linux | ARM64 | [tar.gz]({{ASSET_URL}}/textdock_v{{VERSION}}_linux_arm64.tar.gz) |
| macOS | Intel | [tar.gz]({{ASSET_URL}}/textdock_v{{VERSION}}_darwin_amd64.tar.gz) |
| macOS | Apple Silicon | [tar.gz]({{ASSET_URL}}/textdock_v{{VERSION}}_darwin_arm64.tar.gz) |
| Windows | x86-64 / amd64 | [zip]({{ASSET_URL}}/textdock_v{{VERSION}}_windows_amd64.zip) |

Extract the archive, then run `./textdock` on Linux/macOS or `textdock.exe` on
Windows. Open **http://localhost:18257**.

The binaries are not currently notarized or publisher-signed. The checksums
below verify download integrity, not operating-system code signing.

## Verify a download

Download [SHA256SUMS]({{ASSET_URL}}/SHA256SUMS) alongside your archive.

```sh
# Linux, from the directory containing both files
sha256sum --check --ignore-missing SHA256SUMS
```

On macOS, use `shasum -a 256 path/to/archive.tar.gz` and compare the result with
the matching line in `SHA256SUMS`. On Windows, use PowerShell:

```powershell
Get-FileHash .\textdock_v{{VERSION}}_windows_amd64.zip -Algorithm SHA256
```

## Docker

The published image supports Linux amd64 and arm64:

```sh
docker run --rm --name textdock \
  -p 127.0.0.1:18257:18257 \
  -e TEXTDOCK_TOKEN=textdock-local-development \
  -v textdock-data:/data \
  ghcr.io/fless-lab/textdock:{{VERSION}}
```

Open http://localhost:18257 and enter `textdock-local-development`. This example
uses a development token and exposes the host port only on loopback. For phone
access, follow the [pairing and network guide](/guide/connect).

## Node SDK

The SDK is distributed as a GitHub Release package, with TypeScript declarations
and no third-party runtime dependencies. It runs in Node.js 20.3 or later.

```sh
npm install {{ASSET_URL}}/textdock-sdk-{{VERSION}}.tgz
```

See the [SDK reference](/reference/sdk) for local and production driver setup.

## Build from source

Requirements: Go 1.26+, Node.js 24+, npm and Make.

```sh
git clone https://github.com/fless-lab/TextDock.git
cd TextDock
git checkout v{{VERSION}}
make setup
make build
./bin/textdock
```

## Upgrade

Back up your SQLite database before upgrading, using the currently installed
version. Then stop TextDock and replace the binary or container image. Database
migrations run at startup; older binaries may not understand a newer schema.

Read the [backup and restore guide](/guide/workflow#backup-and-restore) and
[changelog](/project/changelog) for version-specific details.
