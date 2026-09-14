import { defineConfig } from 'vitepress';
import { readFileSync, readdirSync } from 'node:fs';

const version = readFileSync(new URL('../../VERSION', import.meta.url), 'utf8').trim();
const base = '/TextDock/';

export default defineConfig({
  title: 'TextDock',
  titleTemplate: ':title · TextDock',
  description: 'Capture, inspect and test SMS locally. One binary, a phone inbox, delivery simulation and a provider-neutral SDK.',
  lang: 'en-US',
  base,
  srcDir: 'content',
  cleanUrls: false,
  // These are addresses of the reader's local application, not website routes.
  ignoreDeadLinks: [/^https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?(?:\/|$)/],
  lastUpdated: false,
  vite: { server: { port: 18260, strictPort: true }, preview: { port: 18260, strictPort: true } },
  sitemap: { hostname: 'https://fless-lab.github.io/TextDock/' },
  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: `${base}favicon.svg` }],
    ['meta', { name: 'theme-color', content: '#ffffff' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:site_name', content: 'TextDock' }],
  ],
  transformHead({ pageData }) {
    if (pageData.relativePath === '404.md') return [];
    const path = pageData.relativePath.replace(/\.md$/, '.html').replace(/index\.html$/, '');
    return [['link', { rel: 'canonical', href: `https://fless-lab.github.io/TextDock/${path}` }]];
  },
  markdown: { lineNumbers: false },
  themeConfig: {
    logo: '/favicon.svg',
    siteTitle: 'TextDock',
    nav: [
      { text: 'Documentation', link: '/guide/getting-started' },
      { text: 'Downloads', link: '/downloads' },
      { text: 'Roadmap', link: '/project/status' },
      { text: `v${version}`, link: `https://github.com/fless-lab/TextDock/releases/tag/v${version}` },
    ],
    sidebar: [
      { text: 'Start', items: [
        { text: 'Getting started', link: '/guide/getting-started' },
        { text: 'Downloads & installation', link: '/downloads' },
        { text: 'Phone numbers & OTPs', link: '/guide/test-numbers' },
      ] },
      { text: 'Use TextDock', items: [
        { text: 'Projects, CLI & backups', link: '/guide/workflow' },
        { text: 'Connect a phone', link: '/guide/connect' },
        { text: 'Delivery simulation', link: '/guide/simulation' },
        { text: 'Providers & production', link: '/guide/providers' },
        { text: 'Real SMS relay (preview)', link: '/guide/relay' },
        { text: 'Android gateway', link: '/reference/android-gateway' },
        { text: 'Android emulator lab', link: '/guide/device-lab' },
        { text: 'Node SDK', link: '/reference/sdk' },
        { text: 'Native SMS & autofill', link: '/guide/mobile-and-otp' },
      ] },
      { text: 'Reference', items: [
        { text: 'HTTP API', link: '/reference/api' },
        { text: 'OpenAPI contracts', link: '/reference/contracts' },
        { text: 'Architecture', link: '/reference/architecture' },
        { text: 'Security & data', link: '/project/security' },
      ] },
      { text: 'Project', items: [
        { text: 'Current status & next steps', link: '/project/status' },
        { text: 'Versioned roadmap', link: '/project/roadmap' },
        { text: 'Changelog', link: '/project/changelog' },
        { text: 'Contributing', link: '/project/contributing' },
        { text: 'License', link: '/project/license' },
        { text: 'Release process', link: '/project/releases' },
        { text: 'Website publishing', link: '/project/website' },
        { text: 'UI direction', link: '/project/ui' },
        { text: 'Résumé en français', link: '/fr/projet' },
      ] },
      { text: 'Verification records', collapsed: true, items: readdirSync(new URL('../../docs', import.meta.url)).filter(file => /^VALIDATION-v.*\.md$/.test(file)).sort((a, b) => a.localeCompare(b, undefined, { numeric: true })).map(file => { const tag = file.slice('VALIDATION-'.length, -3); return { text: tag, link: `/validation/${tag}` }; }) },
    ],
    search: { provider: 'local' },
    outline: { level: [2, 3], label: 'On this page' },
    socialLinks: [{ icon: 'github', link: 'https://github.com/fless-lab/TextDock' }],
    footer: { message: `<a href="${base}project/license.html">MIT license</a>`, copyright: 'TextDock contributors' },
    docFooter: { prev: 'Previous', next: 'Next' },
  },
});
