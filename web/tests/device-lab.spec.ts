import { expect, test } from '@playwright/test';

test('explicit emulator selection, injection history and physical-device exclusion', async ({ page }, info) => {
  await page.goto('/');
  await page.getByLabel('Server token').fill('textdock-e2e-token-only');
  await page.getByRole('button', { name: 'Unlock inbox' }).click();
  await page.getByRole('button', { name: 'Device lab', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toContainText('physical-test');
  const targets = dialog.getByRole('combobox', { name: 'Target emulator', exact: true });
  await expect(targets).toHaveValue('');
  await expect(dialog.getByRole('button', { name: 'Inject into emulator', exact: true })).toBeDisabled();
  await expect(targets.locator('option')).toHaveCount(2);
  await targets.selectOption('emulator-5554');
  await dialog.getByRole('textbox', { name: 'SMS text', exact: true }).fill('Code 482193\n\n@login.example.test #482193');
  await dialog.getByLabel('Test run ID').fill(`lab-${info.project.name}`);
  await dialog.getByRole('button', { name: 'Inject into emulator', exact: true }).click();
  await expect(dialog.getByRole('status')).toContainText('SMS accepted by emulator-5554');
  await expect(dialog.locator('.lab-history')).toContainText('injected');
  await expect(dialog.getByRole('button', { name: 'Inject into emulator', exact: true })).toBeDisabled();
  await dialog.getByRole('button', { name: 'New attempt', exact: true }).click();
  await dialog.getByRole('textbox', { name: 'SMS text', exact: true }).fill('reject-this-message');
  await dialog.getByRole('button', { name: 'Inject into emulator', exact: true }).click();
  await expect(dialog.getByRole('status')).toContainText('Injection failed');
  await expect(dialog).toContainText('KO: fixture rejected message');
  await page.screenshot({ path: info.outputPath('device-lab.png'), fullPage: true });
  // The browser suite uses a test-only ADB executable, not a real emulator.
});
