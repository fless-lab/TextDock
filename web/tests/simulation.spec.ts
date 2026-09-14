import { expect, test } from '@playwright/test';
import { createServer } from 'node:http';
import twilio from 'twilio';

test('simulate delivery with SDK-verified callbacks, inspect events and replay', async ({ page, request }, testInfo) => {
  const signatures: boolean[] = [];
  let webhookURL = '';
  const receiver = createServer(async (req, res) => {
    let body = ''; for await (const chunk of req) body += chunk;
    signatures.push(twilio.validateRequest('textdock-e2e-webhook-secret', String(req.headers['x-twilio-signature']), webhookURL, Object.fromEntries(new URLSearchParams(body))));
    res.writeHead(204); res.end();
  });
  await new Promise<void>(resolve => receiver.listen(0, '127.0.0.1', resolve));
  const address = receiver.address();
  if (!address || typeof address === 'string') throw new Error('Missing callback listener');
  webhookURL = `http://127.0.0.1:${address.port}/status?test=twilio`;
  try {
    await page.goto('/');
    await page.getByLabel('Server token').fill('textdock-e2e-token-only');
    await page.getByRole('button', { name: 'Unlock inbox' }).click();
    await page.getByRole('button', { name: 'Scenarios', exact: true }).click();
    const name = `Delivery ${testInfo.project.name}`;
    await page.getByLabel('Scenario name').fill(name);
    await page.getByLabel('Delay (ms)').fill('20');
    await page.getByLabel('Delivery webhook URL').fill(webhookURL);
    await page.getByLabel('Webhook format').selectOption('twilio');
    await page.getByRole('button', { name: 'Create scenario', exact: true }).click();
    await expect(page.getByRole('dialog').getByText(name, { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Close dialog' }).click();
    await page.getByRole('button', { name: 'Send test SMS' }).first().click();
    await page.getByRole('dialog').getByRole('combobox', { name: 'Mode', exact: true }).selectOption('simulate');
    await page.getByRole('dialog').getByRole('combobox', { name: 'Scenario', exact: true }).selectOption({ label: name });
    await page.getByRole('button', { name: 'Capture message' }).click();
    await expect(page.locator('.detail-tabs .status-pill')).toHaveText('delivered');
    await page.getByRole('tab', { name: 'Events', exact: true }).click();
    await expect(page.locator('.event-list strong')).toHaveText(['queued', 'sent', 'delivered']);
    await expect(page.locator('.webhook-attempt')).toHaveCount(2);
    expect(signatures).toEqual([true, true]);
    await page.locator('.webhook-attempt summary').first().click();
    await page.getByRole('button', { name: 'Replay callback' }).first().click();
    await expect(page.locator('.webhook-attempt')).toHaveCount(3);
    expect(signatures).toEqual([true, true, true]);
    const headers = { Authorization: 'Bearer textdock-e2e-token-only' };
    const scenario = await request.post('/api/v1/scenarios', { headers, data: { name: 'Rate limit', reject_status: 429, retry_after: 7 } });
    const rejected = await request.post('/api/v1/messages', { headers, data: { to: '+33612345678', from: 'Acme', body: 'Code 123456', scenario_id: (await scenario.json()).id } });
    expect(rejected.status()).toBe(429);
    expect(rejected.headers()['retry-after']).toBe('7');
  } finally { await new Promise<void>(resolve => receiver.close(() => resolve())); }
});
