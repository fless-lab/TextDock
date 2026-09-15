# Releasing TextDock

## First repository publication

The project uses `https://github.com/fless-lab/TextDock.git` as `origin`.
Enable Actions and package publishing, then push the reviewed branch and tag.
Publishing requires remote write access; a local tag alone does not start GitHub
Actions. Authenticate GitHub CLI with `gh auth login` to manage/check releases.

```sh
git push -u origin main
git push origin v0.1.0
```

The release workflow uses the repository's `GITHUB_TOKEN`, with job-scoped
`contents: write` and `packages: write`. It publishes to
`ghcr.io/fless-lab/textdock`. Configure the package's
visibility to public if distributing public images. Enable private vulnerability
reporting and fill in maintainer/project contact details before public launch.

## Cut each subsequent version

1. Complete the version's acceptance criteria in `docs/ROADMAP.md`.
2. Update `VERSION`, `web/package.json`, `site/package.json`, both lockfile root
   versions, `packages/sdk/package.json` and OpenAPI info;
   move Unreleased notes into a dated `CHANGELOG.md` section.
   Also update Android gateway `versionName`, both Android `versionCode` values,
   and the iOS sample's marketing version/build number in `project.yml`.
3. Validate locally:

   ```sh
   make setup
   make check test
    make e2e
    npm --prefix site ci
    npm --prefix site run build
    npm --prefix site test
    node scripts/check-version.mjs v0.6.0
    docker build --build-arg VERSION=v0.6.0 -t textdock:v0.6.0 .
   ```

4. Review `git status`, `git diff` and `git log --oneline -10`. Stage only intended
   sources/docs/lockfiles, then commit. Use focused commits while implementing;
   one release does not require cramming all work into a single commit.
5. Create an annotated tag at the verified commit:

   ```sh
   git tag -a v0.6.0 -m 'TextDock v0.6.0'
   git push origin main
   git push origin v0.6.0
   ```

6. Watch CI, install/smoke-test a produced artifact, and verify the public site.

The commands above use v0.6.0 as an example; substitute the version being cut.

Never advance versions or create tags just because their directory stubs exist.
Do not move a published tag. Fix source regressions in a patch version. An
infrastructure-only workflow failure can be rerun; existing GitHub releases
must be inspected before retrying a partially completed publication.

## Tag pipeline

1. Validate SemVer-shaped tag against VERSION, frontend package/lock metadata,
   and the changelog.
2. Run the reusable CI: formatting/types, Go vet/race tests, UI build, desktop
   and mobile browser tests, SDK contracts, plus the website build, local-link
   checker and website browser/accessibility tests.
3. Cross-compile CGO-free binaries for Linux amd64/arm64, macOS amd64/arm64 and
   Windows amd64. Archive with license, README and changelog.
4. Build/push the Linux amd64/arm64 container image to GHCR.
5. Package the Node SDK as `textdock-sdk-X.Y.Z.tgz`, with source, types and license.
   Build/lint the Android development companion and receiving sample and include
   both APKs in checksums. Run Android receiving-form unit/emulator tests and
   iOS unit/simulator UI tests, then package the iOS simulator application.
   Run the disposable Android emulator integration before publishing device-lab releases.
6. After artifact/image/SDK success, publish GitHub Release with SHA256SUMS and
    generated notes. A hyphenated version is marked prerelease.
7. Deploy the tested Pages artifact for the latest stable release. The shared
   `github-pages` environment permits `v*` tags; prereleases keep the stable site.
   Details: [website publishing](WEBSITE.md).

Checksums detect download corruption; they are not publisher signatures.
Signed provenance is a later distribution milestone. Native runtime tests for
every cross-compiled OS and physical-device SMS checks are not implied by a
successful Linux CI run.
