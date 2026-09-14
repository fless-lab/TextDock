import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { createServer } from 'node:http';
import test from 'node:test';
import { createSMS, fromEnv, SMSError } from '../index.js';

const input = { to: '+33612345678', from: 'Acme', body: 'Your code is 482193', runId: 'test-run' };

async function receiver(t, handler) {
  const server = createServer(async (req, res) => {
    let text = ''; for await (const part of req) text += part;
    try { await handler(req, res, text); }
    catch (error) { res.writeHead(500); res.end(JSON.stringify({ error: error.message })); }
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  t.after(() => { server.closeAllConnections(); server.close(); });
  return `http://127.0.0.1:${server.address().port}`;
}

test('same send interface produces local and Twilio protocol requests', async t => {
  let calls = 0;
  const url = await receiver(t, (req, res, body) => {
    calls++;
    if (req.url === '/api/v1/messages') {
      assert.equal(req.headers.authorization, 'Bearer local-token');
      assert.deepEqual(JSON.parse(body), { to: input.to, from: input.from, body: input.body, inbox: 'checkout', run_id: 'test-run', scenario_id: '', callback_url: '' });
      res.end(JSON.stringify({ id: 'msg_1', status: 'captured' }));
    } else {
      assert.equal(req.url, '/2010-04-01/Accounts/ACtest/Messages.json');
      assert.equal(req.headers.authorization, 'Basic '+Buffer.from('ACtest:secret').toString('base64'));
      assert.deepEqual(Object.fromEntries(new URLSearchParams(body)), { To: input.to, From: input.from, Body: input.body });
      res.end(JSON.stringify({ sid: 'SMtest', status: 'queued' }));
    }
  });
  assert.deepEqual(await createSMS({ baseURL: url, token: 'local-token', inbox: 'checkout' }).send(input), { id: 'msg_1', status: 'captured', provider: 'local' });
  assert.deepEqual(await createSMS({ driver: 'twilio', baseURL: url, accountSid: 'ACtest', authToken: 'secret' }).send(input), { id: 'SMtest', status: 'queued', provider: 'twilio' });
  assert.equal(calls, 2);
});

test('Vonage selects encoding and recognizes errors inside HTTP 200', async t => {
  const types = [];
  const url = await receiver(t, (req, res, raw) => {
    assert.equal(req.url, '/sms/json');
    const body = JSON.parse(raw);
    types.push(body.type);
    assert.equal(body.to, '33612345678');
    assert.equal(body.api_secret, 'secret');
    res.end(JSON.stringify({ 'message-count': '1', messages: [types.length === 3 ? { status: '4', 'error-text': 'Bad credentials' } : { status: '0', 'message-id': 'abc123' }] }));
  });
  const client = createSMS({ driver: 'vonage', baseURL: url, apiKey: 'key', apiSecret: 'secret' });
  assert.equal((await client.send({ ...input, body: 'été' })).id, 'abc123');
  await client.send({ ...input, body: 'Hello 🙂' });
  await assert.rejects(client.send(input), error => error instanceof SMSError && error.code === '4' && !error.uncertain);
  assert.deepEqual(types, ['text', 'unicode', 'text']);
});

test('OVH synchronizes time and signs the exact job request', async t => {
  const timestamp = 1700000000;
  let calls = 0;
  let base;
  base = await receiver(t, (req, res, body) => {
    calls++;
    if (req.url === '/1.0/auth/time') { res.end(JSON.stringify(timestamp)); return; }
    assert.equal(req.url, '/1.0/sms/sms-service/jobs');
    const canonical = ['app-secret', 'consumer', 'POST', base + req.url, body, timestamp].join('+');
    assert.equal(req.headers['x-ovh-signature'], '$1$'+createHash('sha1').update(canonical).digest('hex'));
    assert.equal(req.headers['x-ovh-application'], 'app-key');
    assert.equal(req.headers['x-ovh-timestamp'], String(timestamp));
    assert.deepEqual(JSON.parse(body), { receivers: [input.to], sender: 'Acme', message: input.body, priority: 'high', noStopClause: false });
    res.end(JSON.stringify({ ids: [42], validReceivers: [input.to], invalidReceivers: [], totalCreditsRemoved: 1 }));
  });
  const client = createSMS({ driver: 'ovh', baseURL: base+'/1.0', appKey: 'app-key', appSecret: 'app-secret', consumerKey: 'consumer', service: 'sms-service' });
  assert.deepEqual(await client.send(input), { id: '42', status: 'accepted', provider: 'ovh' });
  assert.equal(calls, 2);
});

test('configuration, HTTP errors and uncertain responses never auto-retry', async t => {
  assert.throws(() => createSMS({ driver: 'unknown' }), /Unsupported/);
  assert.throws(() => fromEnv({ SMS_DRIVER: 'twilio' }), /accountSid/);
  let calls = 0;
  const url = await receiver(t, (req, res) => { calls++; res.writeHead(503, { 'Retry-After': '7' }); res.end('{"error":"Unavailable"}'); });
  await assert.rejects(createSMS({ baseURL: url }).send(input), error => error instanceof SMSError && error.status === 503 && error.retryAfter === '7' && error.uncertain);
  assert.equal(calls, 1);
  await assert.rejects(createSMS({ baseURL: url }).send({ ...input, to: '0612345678' }), TypeError);
  assert.equal(calls, 1);
});
