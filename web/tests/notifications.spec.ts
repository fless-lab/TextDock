import { expect, test as base, chromium, type Page, type APIRequestContext } from '@playwright/test';
import { createECDH, randomBytes } from 'node:crypto';

const admin = { Authorization: 'Bearer textdock-e2e-token-only' };
// Chromium intentionally blocks notifications in off-the-record contexts.
// Use an isolated persistent profile, never the developer's browser profile.
const test = base.extend({
  context: async ({}, use, info) => {
    const context = await chromium.launchPersistentContext(info.outputPath('browser-profile'), {
      channel: 'chromium', headless: true, baseURL: 'http://127.0.0.1:18258',
      viewport: info.project.use.viewport, userAgent: info.project.use.userAgent,
      isMobile: info.project.use.isMobile, hasTouch: info.project.use.hasTouch,
      deviceScaleFactor: info.project.use.deviceScaleFactor,
    });
    await use(context); await context.close();
  },
});
async function pair(page: Page, request: APIRequestContext, run: string) {
  const created = await request.post('/api/v1/pairings', { headers: admin, data: { to: '+12025550123', run_id: run } });
  const { code } = await created.json();
  // Manual pairing supports the separate storage of installed iOS web apps.
  await page.goto('/phone');
  await page.getByLabel('Pairing code or link').fill(code);
  await page.getByRole('button', { name: 'Connect phone', exact: true }).click();
  await expect(page.getByText('Read only', { exact: false })).toBeVisible();
  const token = await page.evaluate(() => localStorage.getItem('textdock-device-token'));
  const session = await request.get('/connect/v1/session', { headers: { Authorization: `Bearer ${token}` } });
  return { token: token!, device: await session.json() };
}

test('manual pairing, scoped push preferences and generic service-worker alerts', async ({ page, context, request }, info) => {
  await context.grantPermissions(['notifications'], { origin: 'http://127.0.0.1:18258' });
  const ecdh = createECDH('prime256v1'); ecdh.generateKeys();
  const subscription = { endpoint: `https://push.invalid/${info.project.name}`, expirationTime: null, keys: { p256dh: ecdh.getPublicKey().toString('base64url'), auth: randomBytes(16).toString('base64url') } };
  // Browser transport registration is stubbed: headless Chromium has no real
  // platform push account. The backend encrypted wire protocol is tested in Go.
  await page.addInitScript(({ subscription }) => {
    function current() {
      const raw = sessionStorage.getItem('push-test-registration');
      if (!raw) return null;
      const data = JSON.parse(raw);
      return { endpoint: subscription.endpoint, options: { applicationServerKey: new Uint8Array(data.key).buffer }, toJSON: () => subscription, unsubscribe: async () => { sessionStorage.removeItem('push-test-registration'); return true; } };
    }
    PushManager.prototype.getSubscription = async function () { return current() as PushSubscription | null; };
    PushManager.prototype.subscribe = async function (options) {
      sessionStorage.setItem('push-test-registration', JSON.stringify({ key: Array.from(new Uint8Array(options!.applicationServerKey as ArrayBuffer)) }));
      return current() as unknown as PushSubscription;
    };
  }, { subscription });
  const { token, device } = await pair(page, request, `push-${info.project.name}`);
  expect(await page.evaluate(() => Notification.permission)).toBe('granted');
  await page.locator('.phone-notifications summary').click();
  await page.getByRole('button', { name: 'Enable notifications' }).click();
  await expect(page.locator('.phone-notifications')).toContainText('notifications enabled');
  const stateResponse = await request.get('/connect/v1/push/state', { headers: { Authorization: `Bearer ${token}` } });
  const state = await stateResponse.json();
  expect(state.subscribed).toBe(true);

  const cdp = await context.newCDPSession(page);
  let registrationId = '';
  cdp.on('ServiceWorker.workerRegistrationUpdated', ({ registrations }) => {
    for (const reg of registrations) if (reg.scopeURL.endsWith('/phone') && !reg.isDeleted) registrationId = reg.registrationId;
  });
  await cdp.send('ServiceWorker.enable');
  await expect.poll(() => registrationId).not.toBe('');
  await cdp.send('ServiceWorker.deliverPushMessage', {
    origin: 'http://127.0.0.1:18258', registrationId,
    data: JSON.stringify({ device_id: device.id, generation: state.generation, kind: 'inbox', body: 'Secret OTP 918273', url: 'https://unrelated.invalid' }),
  });
  const notices = () => page.evaluate(async () => {
    const reg = await navigator.serviceWorker.ready;
    return (await reg.getNotifications({ tag: 'textdock-inbox' })).map(n => ({ title: n.title, body: n.body, data: n.data }));
  });
  await expect.poll(notices).toHaveLength(1);
  const shown = (await notices())[0];
  expect(shown.body).toBe('New message in your TextDock inbox.');
  expect(JSON.stringify(shown)).not.toContain('918273');
  expect(shown.data).not.toHaveProperty('url');

  await request.delete(`/api/v1/devices/${device.id}`, { headers: admin });
  await expect(page.getByRole('alert')).toContainText('This session has ended');
  await expect.poll(notices).toHaveLength(0);
  const revoked = await request.get('/connect/v1/push/state', { headers: { Authorization: `Bearer ${token}` } });
  expect(revoked.status()).toBe(401);
});

test('phone offline fallback contains no cached messages', async ({ page, context, request }, info) => {
  await pair(page, request, `offline-${info.project.name}`);
  await page.evaluate(() => navigator.serviceWorker.ready.then(() => true));
  await expect.poll(() => page.evaluate(() => !!navigator.serviceWorker.controller)).toBe(true);
  await context.setOffline(true);
  await page.reload();
  await expect(page.getByRole('heading', { name: 'TextDock is offline' })).toBeVisible();
  await expect(page.locator('body')).not.toContainText('482193');
  await context.setOffline(false);
  await page.getByRole('link', { name: 'Retry connection' }).click();
  await expect(page.getByText('Read only', { exact: false })).toBeVisible();
});

test('denied browser permission is diagnosed without subscribing', async ({ page, context, request }, info) => {
  await pair(page, request, `denied-${info.project.name}`);
  const cdp = await context.newCDPSession(page);
  await cdp.send('Browser.setPermission', { permission: { name: 'notifications' }, setting: 'denied', origin: 'http://127.0.0.1:18258' });
  await page.locator('.phone-notifications summary').click();
  await page.getByRole('button', { name: 'Refresh notification status' }).click();
  await expect(page.locator('.phone-notifications')).toContainText('blocked permission');
  await expect(page.getByRole('button', { name: 'Enable notifications' })).toBeDisabled();
});
