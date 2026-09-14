import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';
import { readFileSync } from 'node:fs';

const base = '/TextDock/';
const version = readFileSync(new URL('../../VERSION', import.meta.url), 'utf8').trim();

test('homepage, documentation navigation and responsive layout', async ({ page }, info) => {
  const errors: string[] = [];
  const external: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('request', request => { if (/^https?:/.test(request.url()) && new URL(request.url()).hostname !== '127.0.0.1') external.push(request.url()); });
  await page.goto(base);
  await expect(page.getByRole('heading', { level: 1 })).toContainText('SMS inbox.');
  await expect(page.locator('.release-line')).toContainText(`v${version}`);
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false);
  await page.screenshot({ path: info.outputPath('home.png'), fullPage: true });
  await page.getByRole('link', { name: 'Get started' }).click();
  await expect(page).toHaveURL(/\/TextDock\/guide\/getting-started\.html$/);
  await expect(page.locator('.vp-doc')).toContainText('18257');
  expect(errors).toEqual([]);
  expect(external).toEqual([]);
});

test('deep links, release downloads and raw API contracts', async ({ page, request }) => {
  for (const [route, heading] of [
    ['guide/connect.html', 'Connect a phone'],
    ['guide/test-numbers.html', 'Phone numbers and OTPs in tests'],
    ['reference/sdk.html', '@textdock/sdk'],
    ['project/status.html', 'Current status and next steps'],
  ]) {
    await page.goto(base + route);
    await expect(page.getByRole('heading', { level: 1 })).toContainText(heading);
  }
  await page.reload();
  await expect(page.locator('.vp-doc')).toContainText('not finished');
  await page.goto(base + 'downloads.html');
  await expect(page.locator(`a[href$="_linux_amd64.tar.gz"]`)).toHaveAttribute('href', `https://github.com/fless-lab/TextDock/releases/download/v${version}/textdock_v${version}_linux_amd64.tar.gz`);
  await expect(page.locator('.vp-doc')).toContainText(`textdock-sdk-${version}.tgz`);
  const schema = await request.get(base + 'api/openapi.yaml');
  expect(schema.status()).toBe(200);
  expect(await schema.text()).toContain('openapi: 3.1.0');
});

test('local search finds documentation', async ({ page }) => {
  await page.goto(base);
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  const search = page.locator('#localsearch-input');
  await search.fill('pairing');
  await expect(page.locator('.VPLocalSearchBox')).toContainText(/Connect a phone|Phone pairing/);
  await page.keyboard.press('Escape');
  await expect(search).not.toBeVisible();
});

test('light and dark pages pass automated accessibility checks', async ({ page }, info) => {
  await page.goto(base);
  let report = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
  expect(report.violations).toEqual([]);
  await page.addInitScript(() => localStorage.setItem('vitepress-theme-appearance', 'dark'));
  await page.reload();
  await expect(page.locator('html')).toHaveClass(/dark/);
  await page.screenshot({ path: info.outputPath('home-dark.png'), fullPage: true });
  report = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
  expect(report.violations).toEqual([]);
  await page.goto(base + 'guide/test-numbers.html');
  report = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
  expect(report.violations).toEqual([]);
});
