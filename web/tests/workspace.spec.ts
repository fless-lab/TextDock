import { expect, test } from '@playwright/test';

test('create a project, tag a favorite, export and bulk delete', async ({ page }, testInfo) => {
  await page.goto('/');
  await page.getByLabel('Server token').fill('textdock-e2e-token-only');
  await page.getByRole('button', { name: 'Unlock inbox' }).click();
  await page.getByRole('button', { name: 'Manage projects' }).click();
  await page.getByLabel('New project name').fill(`Checkout ${testInfo.project.name}`);
  await page.getByRole('button', { name: 'Create project', exact: true }).click();
  await expect(page.getByLabel('Inbox', { exact: true })).toHaveValue(/^inbox_/);
  await page.getByRole('button', { name: 'Send test SMS' }).first().click();
  await page.getByRole('textbox', { name: 'Message', exact: true }).fill('Workspace code 482193 🙂');
  await page.getByRole('button', { name: 'Capture message' }).click();
  await expect(page.locator('.sms-bubble')).toHaveText('Workspace code 482193 🙂');
  await expect(page.locator('.encoding-note')).toContainText('🙂');
  await page.getByRole('button', { name: 'Add to favorites' }).click();
  await expect(page.getByRole('button', { name: 'Remove from favorites' })).toBeVisible();
  await page.getByLabel('Tags', { exact: true }).fill('signup, regression');
  await page.getByRole('button', { name: 'Save tags' }).click();
  await expect(page.getByLabel('Tags', { exact: true })).toHaveValue('regression, signup');
  await page.getByRole('button', { name: 'Favorites', exact: true }).click();
  await expect(page.locator('.message-row')).toHaveCount(1);
  const download = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Export page' }).click();
  expect((await download).suggestedFilename()).toBe('textdock.csv');
  await page.getByLabel('Select page').check();
  await page.getByRole('button', { name: 'Delete selected (1)' }).click();
  await expect(page.locator('.message-row')).toHaveCount(0);
});
