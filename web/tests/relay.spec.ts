import { expect, test } from '@playwright/test';

test('enroll a gateway, queue one real-mode intent and record its result', async ({ page, request }, info) => {
  await page.goto('/');
  await page.getByLabel('Server token').fill('textdock-e2e-token-only');
  await page.getByRole('button', { name: 'Unlock inbox' }).click();
  await page.getByRole('button', { name: 'Relay', exact: true }).click();
  await page.getByLabel('Gateway name').fill(`Test gateway ${info.project.name}`);
  await page.getByRole('button', { name: 'Create gateway token' }).click();
  const token = await page.getByLabel('Gateway token', { exact: true }).inputValue();
  await page.getByLabel('Application hostname').fill('login.example.test');
  await page.getByRole('button', { name: 'Format message' }).click();
  await expect(page.getByRole('dialog').locator('pre').last()).toContainText('@login.example.test #482193');
  await page.getByRole('button', { name: 'Close dialog' }).click();
  await page.getByRole('button', { name: 'Send test SMS' }).first().click();
  await page.getByRole('dialog').getByRole('combobox', { name: 'Mode', exact: true }).selectOption('relay');
  await page.getByRole('button', { name: 'Send real SMS' }).click();
  await expect(page.locator('.detail-tabs .status-pill')).toHaveText('queued');
  const claimed = await request.post('/relay/v1/jobs/claim', { headers: { Authorization: `Bearer ${token}` } });
  expect(claimed.status()).toBe(200);
  const job = await claimed.json();
  const result = await request.post('/relay/v1/jobs/result', { headers: { Authorization: `Bearer ${token}` }, data: { id: job.id, lease_token: job.lease_token, state: 'sent' } });
  expect(result.status()).toBe(204);
  await expect(page.locator('.detail-tabs .status-pill')).toHaveText('sent');
  await page.getByRole('tab', { name: 'Events', exact: true }).click();
  await expect(page.getByRole('heading', { name: 'Real SMS dispatch' })).toBeVisible();
  await expect(page.locator('.event-list strong')).toHaveText(['queued', 'dispatching', 'sent']);
  // This is a protocol simulator: no Android device or mobile network is used.
  const forbidden = await request.get('/api/v1/messages', { headers: { Authorization: `Bearer ${token}` } });
  expect(forbidden.status()).toBe(403);
});
