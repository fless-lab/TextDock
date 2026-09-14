import { expect, test } from '@playwright/test';
import twilio from 'twilio';

test('capture, inspect, find and delete a message on desktop and phone', async ({ page }, testInfo) => {
  const errors: string[] = [];
  page.on('pageerror', e => errors.push(e.message));
  const body = `Your ${testInfo.project.name} sign-in code is 729184.`;
  await page.goto('/');
  await page.getByLabel('Server token').fill('textdock-e2e-token-only');
  await page.getByRole('button', { name: 'Unlock inbox' }).click();
  await expect(page.getByRole('heading', { name: 'Message inbox' })).toBeVisible();
  await page.getByRole('button', { name: 'Send test SMS' }).first().click();
  await page.getByRole('textbox', { name: 'Message', exact: true }).fill(body);
  await page.getByLabel('Test run ID').fill(`browser-${testInfo.project.name}`);
  await page.getByRole('button', { name: 'Capture message' }).click();
  await expect(page.locator('.sms-bubble')).toHaveText(body);
  await expect(page.locator('.otp-card strong')).toHaveText('729184');
  await page.getByRole('tab', { name: 'Raw JSON' }).click();
  await expect(page.locator('.raw-view pre')).toContainText(`browser-${testInfo.project.name}`);
  await page.getByRole('tab', { name: 'Message', exact: true }).click();
  await page.getByRole('button', { name: 'Toggle color theme' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', /dark|light/);
  await page.screenshot({ path: testInfo.outputPath('inbox.png'), fullPage: true });
  const overflowing = await page.evaluate(() => document.documentElement.scrollWidth > innerWidth);
  expect(overflowing).toBe(false);
  if (testInfo.project.name === 'mobile') await page.getByRole('button', { name: 'Back to messages' }).click();
  await page.getByLabel('Search messages').fill('a-message-that-does-not-exist');
  await expect(page.getByRole('heading', { name: 'No matching messages' })).toBeVisible();
  await page.getByLabel('Search messages').fill(body);
  await page.getByRole('button').filter({ hasText: body }).click();
  await page.getByRole('button', { name: 'Delete message' }).click();
  await expect(page.getByRole('heading', { name: 'No matching messages' })).toBeVisible();
  expect(errors).toEqual([]);
});

test('official Twilio Node SDK creates a message through the local endpoint', async ({ request }) => {
  const client = twilio('AC' + '0'.repeat(32), 'textdock-e2e-token-only');
  // This is the public SDK's API-domain override; no real Twilio call occurs.
  client.api.baseUrl = 'http://127.0.0.1:18258';
  const message = await client.messages.create({ to: '+33612345678', from: 'Acme', body: 'SDK verification 654321' });
  expect(message.sid).toMatch(/^SM[0-9a-f]{32}$/);
  expect(message.status).toBe('queued');
  const response = await request.get('/api/v1/messages?q=SDK%20verification', { headers: { Authorization: 'Bearer textdock-e2e-token-only' } });
  expect(response.ok()).toBe(true);
  const data = await response.json();
  expect(data.messages[0].source).toBe('twilio');
  expect(data.messages[0].analysis.otp).toBe('654321');
});
