import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

test('user login, project roles, live session revocation and password change', async ({ page, browser, request, baseURL }, testInfo) => {
  const admin = { Authorization: 'Bearer textdock-e2e-token-only' };
  const suffix = `${testInfo.project.name}_${Date.now().toString(36)}`;
  const username = `user_${suffix}`;
  const password = 'correct horse battery staple';
  const projectResponse = await request.post('/api/v1/projects', { headers: admin, data: { name: `Team ${suffix}` } });
  expect(projectResponse.status()).toBe(201);
  const { project, inbox } = await projectResponse.json();
  const outsideResponse = await request.post('/api/v1/projects', { headers: admin, data: { name: `Outside ${suffix}` } });
  const outside = await outsideResponse.json();
  for (const [box, body] of [[inbox.id, `Team code 482193 ${suffix}`], [outside.inbox.id, `Outside secret ${suffix}`]]) {
    expect((await request.post('/api/v1/messages', { headers: admin, data: { inbox: box, to: '+12025550123', from: 'Team', body } })).status()).toBe(201);
  }
  await page.goto('/');
  await page.getByLabel('Server token').fill('textdock-e2e-token-only');
  await page.getByRole('button', { name: 'Unlock inbox' }).click();
  await page.getByRole('button', { name: 'Manage projects' }).click();
  await page.getByRole('button', { name: 'Manage team', exact: true }).click();
  await page.getByLabel('Username', { exact: true }).fill(username);
  await page.getByLabel('Display name').fill('Test teammate');
  await page.getByLabel('Initial password').fill(password);
  await page.getByRole('button', { name: 'Create user', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('User created');
  await page.getByRole('combobox', { name: 'Project', exact: true }).selectOption(project.id);
  await page.getByLabel('Member username').fill(username);
  await page.getByRole('combobox', { name: 'Role', exact: true }).selectOption('viewer');
  await page.getByRole('button', { name: 'Assign role' }).click();
  await expect(page.getByRole('status')).toHaveText('Project role saved.');
  expect((await new AxeBuilder({ page }).include('dialog').withTags(['wcag2a', 'wcag2aa']).analyze()).violations).toEqual([]);

  const context = await browser.newContext({ baseURL, viewport: page.viewportSize() || undefined });
  try {
    const member = await context.newPage();
    await member.goto('/');
    await member.getByRole('button', { name: 'Sign in with a user account' }).click();
    async function signIn(secret: string) {
      await member.getByLabel('Username', { exact: true }).fill(username);
      await member.getByLabel('Password', { exact: true }).fill(secret);
      await member.getByRole('button', { name: 'Sign in', exact: true }).click();
      await expect(member.getByLabel('Inbox', { exact: true })).toHaveValue(inbox.id);
    }
    async function backToInbox() { const back = member.getByRole('button', { name: 'Back to messages' }); if (await back.isVisible()) await back.click(); }
    await signIn(password);
    await expect(member.locator('.message-row')).toHaveCount(1);
    await expect(member.locator('.message-row')).toContainText(`Team code 482193 ${suffix}`);
    await expect(member.getByRole('button', { name: 'Send test SMS' }).first()).toBeDisabled();
    await expect(member.getByRole('button', { name: 'Relay', exact: true })).toHaveCount(0);
    expect(await member.getByLabel('Inbox', { exact: true }).locator('option').count()).toBe(1);
    await member.locator('.message-row').click();
    await expect(member.getByRole('button', { name: 'Delete message', exact: true })).toBeDisabled();
    await member.getByRole('tab', { name: 'Events', exact: true }).click();
    await expect(member.getByRole('heading', { name: 'Message history' })).toBeVisible();
    await expect(member.getByRole('heading', { name: 'Webhook attempts' })).toHaveCount(0);
    await expect(member.locator('.error-banner')).toHaveCount(0);
    await backToInbox();
    await member.getByRole('button', { name: 'Your account', exact: true }).click();
    await expect(member.getByText('This session', { exact: true })).toBeVisible();
    expect((await new AxeBuilder({ page: member }).include('dialog').withTags(['wcag2a', 'wcag2aa']).analyze()).violations).toEqual([]);

    await page.getByRole('combobox', { name: 'Role', exact: true }).selectOption('member');
    await page.getByRole('button', { name: 'Assign role' }).click();
    await expect(member.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible();
    await expect(member.locator('.sms-bubble')).toHaveCount(0);
    expect(await member.evaluate(() => sessionStorage.getItem('textdock-token'))).toBeNull();
    await signIn(password);
    await member.getByRole('button', { name: 'Send test SMS' }).first().click();
    await member.getByRole('textbox', { name: 'Message', exact: true }).fill('Member captured 928471');
    await member.getByRole('button', { name: 'Capture message', exact: true }).click();
    await expect(member.locator('.sms-bubble')).toHaveText('Member captured 928471');
    await backToInbox();
    await member.getByRole('button', { name: 'Your account', exact: true }).click();
    await member.getByLabel('Current password').fill(password);
    const nextPassword = 'updated correct horse battery staple';
    await member.getByLabel('New password', { exact: true }).fill(nextPassword);
    await member.getByRole('button', { name: 'Change password and sign out' }).click();
    await expect(member.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible();
    await signIn(nextPassword);
    await member.getByRole('button', { name: 'Your account', exact: true }).click();
    await member.getByRole('button', { name: 'Sign out', exact: true }).click();
    await expect(member.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible();
    expect(await member.evaluate(() => sessionStorage.getItem('textdock-token'))).toBeNull();
  } finally { await context.close(); }
});
