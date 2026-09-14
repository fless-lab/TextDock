import { expect, test } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

test('inbox and modal meet automated WCAG A/AA checks', async ({ page }) => {
  await page.goto('/');
  await page.getByLabel('Server token').fill('textdock-e2e-token-only');
  await page.getByRole('button', { name: 'Unlock inbox' }).click();
  await expect(page.getByRole('heading', { name: 'Message inbox' })).toBeVisible();
  let result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
  expect(result.violations).toEqual([]);
  await page.getByRole('button', { name: 'Send test SMS' }).first().click();
  result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21aa']).analyze();
  expect(result.violations).toEqual([]);
});
