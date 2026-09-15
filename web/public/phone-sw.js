/* TextDock phone service worker: no SMS, OTP or credential caching. */
const database = 'textdock-phone-state';
let pending = Promise.resolve();
function serial(work) { pending = pending.catch(() => {}).then(work); return pending; }
function binding(value) {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(database, 1);
    request.onupgradeneeded = () => request.result.createObjectStore('settings');
    request.onerror = () => reject(request.error);
    request.onsuccess = () => {
      const db = request.result;
      const tx = db.transaction('settings', value === undefined ? 'readonly' : 'readwrite');
      const operation = value === undefined ? tx.objectStore('settings').get('binding') : tx.objectStore('settings').put(value, 'binding');
      let result;
      operation.onsuccess = () => { result = operation.result; };
      tx.oncomplete = () => { db.close(); resolve(result); };
      tx.onerror = () => { db.close(); reject(tx.error); };
    };
  });
}
self.addEventListener('install', event => event.waitUntil(self.skipWaiting()));
self.addEventListener('activate', event => event.waitUntil(self.clients.claim()));
self.addEventListener('message', event => {
  if (event.data?.type !== 'textdock-bind' || !event.source?.url) return;
  const source = new URL(event.source.url);
  if (source.origin !== self.location.origin || source.pathname !== '/phone') return;
  const deviceId = typeof event.data.deviceId === 'string' ? event.data.deviceId : '';
  const generation = typeof event.data.generation === 'string' ? event.data.generation : '';
  if (deviceId.length > 128 || generation.length > 128) return;
  event.waitUntil(serial(async () => {
    const previous = await binding();
    const expiresAt = Number(event.data.expiresAt) || 0;
    const enabled = !!deviceId && !!generation && expiresAt > Date.now();
    await binding({ deviceId, generation, enabled, expiresAt });
    if (!enabled || previous?.deviceId !== deviceId || previous?.generation !== generation) {
      for (const notification of await self.registration.getNotifications()) notification.close();
    }
    event.ports[0]?.postMessage({ ok: true });
  }).catch(() => event.ports[0]?.postMessage({ ok: false })));
});
self.addEventListener('push', event => {
  event.waitUntil(serial(async () => {
    let payload;
    try { payload = event.data?.json(); } catch { return; }
    const state = await binding();
    if (!state?.enabled || state.expiresAt <= Date.now() || payload?.device_id !== state.deviceId || payload?.generation !== state.generation) return;
    await self.registration.showNotification('TextDock', {
      body: payload.kind === 'test' ? 'Test notification. Your paired inbox is ready.' : 'New message in your TextDock inbox.',
      icon: '/phone-icon-192.png', badge: '/phone-badge-96.png',
      tag: 'textdock-inbox', renotify: false,
      data: { deviceId: state.deviceId, generation: state.generation },
    });
  }));
});
self.addEventListener('notificationclick', event => {
  event.notification.close();
  event.waitUntil((async () => {
    const state = await binding();
    if (!state?.enabled || state.expiresAt <= Date.now() || event.notification.data?.deviceId !== state.deviceId || event.notification.data?.generation !== state.generation) return;
    for (const client of await self.clients.matchAll({ type: 'window', includeUncontrolled: true })) {
      const url = new URL(client.url);
      if (url.origin === self.location.origin && url.pathname === '/phone') { await client.focus(); client.postMessage({ type: 'textdock-refresh' }); return; }
    }
    await self.clients.openWindow('/phone');
  })());
});
self.addEventListener('fetch', event => {
  if (event.request.mode !== 'navigate' || new URL(event.request.url).pathname !== '/phone') return;
  event.respondWith(fetch(event.request).catch(() => new Response(`<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>TextDock offline</title><style>body{font:16px/1.7 Arial,Helvetica,sans-serif;margin:48px auto;padding:24px;max-width:480px}h1{font-size:24px}a{color:#24588a}</style><main><h1>TextDock is offline</h1><p>Reconnect to the server’s Wi-Fi or VPN, or start the local server, then try again.</p><p>No SMS or verification codes are cached by this offline page.</p><a href="/phone">Retry connection</a></main></html>`, { headers: { 'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store', 'Content-Security-Policy': "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'" } })));
});
