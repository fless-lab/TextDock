import { expect, test } from '@playwright/test';

test('a late capture response cannot repopulate another user session', async ({ page, request }, testInfo) => {
  const headers = { Authorization: 'Bearer textdock-e2e-token-only' };
  const password = 'correct horse battery staple';
  const suffix = `${testInfo.project.name}_${Date.now().toString(36)}`;
  const accounts: { username: string; inbox: string }[] = [];
  for (const prefix of ['first', 'second']) {
    const username = `${prefix}_${suffix}`;
    const created = await request.post('/api/v1/projects', { headers, data: { name: username } });
    const { project, inbox } = await created.json();
    expect((await request.post('/api/v1/team/users', { headers, data: { username, name: username, password } })).status()).toBe(201);
    expect((await request.put('/api/v1/team/memberships', { headers, data: { project_id: project.id, username, role: 'member' } })).status()).toBe(204);
    accounts.push({ username, inbox: inbox.id });
  }
  await page.goto('/');
  await page.getByRole('button', { name: 'Sign in with a user account' }).click();
  async function login(account: typeof accounts[number]) {
    await page.getByLabel('Username', { exact: true }).fill(account.username);
    await page.getByLabel('Password', { exact: true }).fill(password);
    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    await expect(page.getByLabel('Inbox', { exact: true })).toHaveValue(account.inbox);
  }
  await login(accounts[0]);
  let release!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  let received!: () => void;
  const captured = new Promise<void>(resolve => { received = resolve; });
  await page.route('**/api/v1/messages', async route => {
    if (route.request().method() !== 'POST') { await route.continue(); return; }
    const response = await route.fetch();
    received();
    await held;
    await route.fulfill({ response });
  });
  try {
    const completed = page.waitForResponse(response => response.url().endsWith('/api/v1/messages') && response.request().method() === 'POST');
    await page.getByRole('button', { name: 'Send test SMS' }).first().click();
    await page.getByRole('textbox', { name: 'Message', exact: true }).fill('First account private response');
    await page.getByRole('button', { name: 'Capture message', exact: true }).click();
    await captured;
    await page.getByRole('button', { name: 'Close dialog' }).click();
    await page.getByRole('button', { name: 'Your account', exact: true }).click();
    await page.getByRole('button', { name: 'Sign out', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible();
    await login(accounts[1]);
    release();
    await (await completed).finished();
    await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    await expect(page.getByLabel('Inbox', { exact: true })).toHaveValue(accounts[1].inbox);
    await expect(page.locator('.message-row')).toHaveCount(0);
    await expect(page.locator('.sms-bubble')).toHaveCount(0);
    await expect(page.getByText('First account private response', { exact: true })).toHaveCount(0);
  } finally { release(); }
});
