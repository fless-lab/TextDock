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
2. Update `VERSION`, `web/package.json`, lockfile root versions and OpenAPI info;
   move Unreleased notes into a dated `CHANGELOG.md` section.
3. Validate locally:

   ```sh
   make setup
   make check test
   make e2e
   node scripts/check-version.mjs v0.2.0
   docker build --build-arg VERSION=v0.2.0 -t textdock:v0.2.0 .
   ```

4. Review `git status`, `git diff` and `git log --oneline -10`. Stage only intended
   sources/docs/lockfiles, then commit. Use focused commits while implementing;
   one release does not require cramming all work into a single commit.
5. Create an annotated tag at the verified commit:

   ```sh
   git tag -a v0.2.0 -m 'TextDock v0.2.0'
   git push origin main
   git push origin v0.2.0
   ```

6. Watch CI and install/smoke-test a produced artifact, including the container.

Never advance versions or create tags just because their directory stubs exist.
Do not move a published tag. Fix source regressions in a patch version. An
infrastructure-only workflow failure can be rerun; existing GitHub releases
must be inspected before retrying a partially completed publication.

## Tag pipeline

1. Validate SemVer-shaped tag against VERSION, frontend package/lock metadata,
   and the changelog.
2. Run the reusable CI: formatting/types, Go vet/race tests, UI build, desktop
   and mobile browser tests, official Twilio Node SDK contract test.
3. Cross-compile CGO-free binaries for Linux amd64/arm64, macOS amd64/arm64 and
   Windows amd64. Archive with license, README and changelog.
4. Build/push the Linux amd64/arm64 container image to GHCR.
5. After artifact/image success, publish GitHub Release with SHA256SUMS and
   generated notes. A hyphenated version is marked prerelease.

Checksums detect download corruption; they are not publisher signatures.
Signed provenance is a later distribution milestone. Native runtime tests for
every cross-compiled OS and physical-device SMS checks are not implied by a
successful Linux CI run.
