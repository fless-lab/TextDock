# v0.5.1 website validation

- Production documentation build succeeds, with repository Markdown as the
  canonical source and generated content excluded from Git.
- Static checking verifies project-base paths, local links, anchors and assets.
- Eight Playwright checks pass across desktop and phone viewports: homepage,
  deep-link navigation, release downloads, raw OpenAPI files, local search,
  responsive layout and automated light/dark accessibility checks.
- Homepage requests remain on the local preview origin; no external search,
  analytics or font service is contacted by the page.
- Release metadata supplies the website version and matching artifact URLs.
- GitHub Pages is configured for Actions, with release-tag deployment allowed.

The release workflow deploys the tested artifact after stable-release publication.
Public URL/deep-link and version checks are performed after that deployment;
local preview tests alone do not constitute verification of GitHub hosting.
