// Exercise the built executable's defaults, port conflicts and startup policy.
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { once } from 'node:events';
import { setTimeout as delay } from 'node:timers/promises';

const binary = process.env.TEXTDOCK_BIN || new URL('../bin/textdock', import.meta.url).pathname;
const env = { ...process.env, TEXTDOCK_LISTEN: '', TEXTDOCK_DB: '', TEXTDOCK_TOKEN: '', TEXTDOCK_CONFIG: '', TEXTDOCK_RETENTION: '', TEXTDOCK_OTP_PATTERN: '', TEXTDOCK_PUBLIC_URL: '', TEXTDOCK_RELAY_DRIVER: '', TEXTDOCK_ADB_ENABLED: '' };
const server = spawn(binary, ['--db', ':memory:'], { env, stdio: ['ignore', 'ignore', 'pipe'] });
let logs = '';
server.stderr.on('data', chunk => { logs += chunk; });
server.on('error', error => { logs += error.message; });
try {
  for (let attempt = 0; attempt < 100 && !logs.includes('TextDock ready'); attempt++) {
    assert.equal(server.exitCode, null, logs);
    await delay(50);
  }
  assert.match(logs, /url=http:\/\/127\.0\.0\.1:18257\b/, 'Default listen port must be 18257');
  const base = 'http://127.0.0.1:18257';
  const health = await fetch(`${base}/healthz`);
  assert.equal(health.status, 200);
  const ui = await fetch(base);
  assert.match(await ui.text(), /<title>TextDock/);
  const create = await fetch(`${base}/api/v1/messages`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ to: '+33612345678', from: 'Smoke', body: 'Code 123456', run_id: 'binary-smoke' }),
  });
  assert.equal(create.status, 201);
  const otp = await fetch(`${base}/api/v1/otp?to=%2B33612345678&run_id=binary-smoke`);
  assert.equal((await otp.json()).code, '123456');

  const conflict = spawnSync(binary, ['--db', ':memory:'], { env, encoding: 'utf8', timeout: 5000 });
  assert.equal(conflict.status, 1);
  assert.match(conflict.stderr, /bind|address already in use/);

  const network = spawnSync(binary, ['--listen', '0.0.0.0:0', '--db', ':memory:'], { env, encoding: 'utf8', timeout: 5000 });
  assert.equal(network.status, 1);
  assert.match(network.stderr, /TEXTDOCK_TOKEN/);
  console.log('Binary smoke passed: port 18257, embedded UI, capture, OTP, bind conflict, network token policy.');
} finally {
  if (server.exitCode === null) {
    const exited = once(server, 'exit');
    server.kill('SIGTERM');
    await exited;
  }
}
