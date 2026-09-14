# Public website and release publishing

The public site is **https://fless-lab.github.io/TextDock/**. It provides the
product overview, installation/download links, guides, API contracts, SDK
documentation, project status and release records. It is a static documentation
site, not the future hosted SMS inbox service.

## One source of documentation

Guides remain in `docs/`, with the root README, contributor/security documents
and SDK README as canonical sources. `site/scripts/prepare.mjs` maps them into
website routes, rewrites relative Markdown links and copies OpenAPI files. The
site-specific landing page, downloads and status overview live in `site/pages/`.

Generated `site/content/` is not committed. Every documentation file must have a
route; adding an unmapped Markdown guide fails the build instead of silently
omitting it from the website.

The build reads `VERSION` to produce matching release/download links. Local
search indexes the built documentation; no hosted search service, analytics or
third-party font service is required. The site supports mobile navigation,
light/dark themes, keyboard search, a sitemap and canonical page URLs.

## Preview and verification

```sh
npm --prefix site ci
npm --prefix site run dev
```

After editing canonical Markdown, rerun `npm --prefix site run prepare:docs`
or restart the dev command to refresh the generated pages.

For the production build and browser checks:

```sh
npm --prefix site run build
npm --prefix site exec -- playwright install chromium
npm --prefix site test
```

Preview runs on port 18260 under `/TextDock/`. The static check validates local
links, anchors, assets and the GitHub project base path. Browser tests cover
navigation, downloads, local search, phone layout and automated accessibility.

## Release integration

CI builds and tests the site from the same commit as the application. Stable
tag releases publish the application/SDK artifacts and then deploy the tested
Pages artifact using GitHub Actions. Prereleases do not replace the stable site.
The deployment checks GitHub's latest stable release to avoid an older rerun
overwriting newer documentation.

GitHub Pages must use **GitHub Actions** as its publishing source. The
`github-pages` environment must permit release tags; the deployment job needs
`pages: write` and `id-token: write`. Deployment uses a shared concurrency group.
Site build failures block a release's checks. A Pages publishing failure leaves
the existing site available and is visible in the release workflow.

For recovery without republishing application artifacts, run the **Redeploy
documentation** workflow manually. It resolves the latest stable release,
rebuilds and tests its website, then deploys it. The selected release must include
the website sources (v0.5.1 or later).

After a release, verify the public homepage, a deep documentation link and the
versioned download links. The domain can be changed later, but `base`, canonical
URLs, sitemap and tests must be updated together.

The **Verify public documentation** workflow checks the live homepage, deep
links, release download URLs and OpenAPI version from a GitHub runner. It can be
run independently, and is invoked after deployments. Short retries account for
Pages/CDN propagation. This also permits verification when a developer's local
network filters `github.io`.
