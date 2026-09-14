// Runs only against the disposable Android emulator provisioned by CI.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { once } from 'node:events';
import { setTimeout as delay } from 'node:timers/promises';

const adb = process.env.ADB || 'adb';
const serial = process.env.TEXTDOCK_TEST_SERIAL;
assert.match(serial || '', /^emulator-\d+$/, 'An explicit CI emulator serial is required');
const token = 'emulator-ci-only-token';
const server = spawn('./bin/textdock', ['--listen', '127.0.0.1:18261', '--db', ':memory:'], {
  env: { ...process.env, TEXTDOCK_TOKEN: token, TEXTDOCK_CONFIG: '', TEXTDOCK_RELAY_DRIVER: '', TEXTDOCK_PUBLIC_URL: '', TEXTDOCK_RETENTION: '', TEXTDOCK_ADB_ENABLED: 'true', TEXTDOCK_ADB_PATH: adb },
  stdio: ['ignore', 'ignore', 'pipe'],
});
let logs = '';
server.stderr.on('data', data => { logs += data; });
const base = 'http://127.0.0.1:18261/api/v1';
const headers = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
try {
  for (let n = 0; n < 100 && !logs.includes('TextDock ready'); n++) { assert.equal(server.exitCode, null, logs); await delay(100); }
  assert.match(logs, /TextDock ready/);
  const status = await (await fetch(base + '/lab/status', { headers })).json();
  assert.equal(status.available, true, JSON.stringify(status));
  assert.ok(status.devices.some(d => d.serial === serial && d.can_inject));
  let sim = '';
  for (let n = 0; n < 60; n++) {
    sim = execFileSync(adb, ['-s', serial, 'shell', 'getprop', 'gsm.sim.state'], { encoding: 'utf8', timeout: 10000 });
    if (/READY|LOADED/.test(sim)) break;
    await delay(1000);
  }
  assert.match(sim, /READY|LOADED/, 'Virtual SIM must be ready');
  const controlResponse = await fetch(base + '/lab/injections', { method: 'POST', headers, body: JSON.stringify({ serial, from: '+12025550100', to: '+12025550123', body: 'TextDock control message', idempotency_key: 'control-message' }) });
  const control = await controlResponse.json();
  assert.equal(control.injection?.status, 'injected', JSON.stringify(control));
  const body = 'TextDock code "482193"; $literal \\backslash café 漢字🙂\n\n@login.example.test #482193';
  const payload = { serial, inbox: 'local', from: '+12025550100', to: '+12025550123', body, run_id: 'real-emulator-ci', idempotency_key: 'real-emulator-ci' };
  const response = await fetch(base + '/lab/injections', { method: 'POST', headers, body: JSON.stringify(payload) });
  assert.equal(response.status, 201);
  const result = await response.json();
  assert.equal(result.injection.status, 'injected', JSON.stringify(result));
  assert.equal(result.message.body, body);
  const repeat = await (await fetch(base + '/lab/injections', { method: 'POST', headers, body: JSON.stringify(payload) })).json();
  assert.equal(repeat.replayed, true);
  assert.equal(repeat.message.id, result.message.id);
  await delay(1500);

  // Google APIs userdebug images allow root on disposable emulators. This is
  // test-only inspection; the product never roots devices or reads SMS inboxes.
  execFileSync(adb, ['-s', serial, 'root'], { encoding: 'utf8', timeout: 15000 });
  execFileSync(adb, ['-s', serial, 'wait-for-device'], { timeout: 30000 });
  let inbox = '';
  for (let n = 0; n < 30; n++) {
    inbox = execFileSync(adb, ['-s', serial, 'shell', 'content', 'query', '--uri', 'content://sms/inbox', '--projection', 'address:body'], { encoding: 'utf8', timeout: 15000 });
    if (inbox.includes(body)) break;
    await delay(1000);
  }
  if (!inbox.includes(body)) {
    const radio = execFileSync(adb, ['-s', serial, 'logcat', '-b', 'radio', '-d', '-t', '1000'], { encoding: 'utf8', timeout: 15000 });
    console.log(radio.split('\n').filter(line => /sms|pdu|inbound|cmt/i.test(line)).join('\n'));
  }
  assert.ok(inbox.includes('TextDock control message'), `Control SMS was not received: ${inbox}`);
  assert.ok(inbox.includes(body), `SMS content did not survive console encoding: ${inbox}`);
  const occurrences = inbox.split('TextDock code "482193"').length - 1;
  assert.equal(occurrences, 1, 'Idempotent replay must not create a second SMS');
  console.log('Real Android emulator verified: explicit target, exact Unicode/newline/quote/backslash content, SMS inbox entry and no duplicate replay.');
} finally {
  if (server.exitCode === null) { const exit = once(server, 'exit'); server.kill('SIGTERM'); await exit; }
}
