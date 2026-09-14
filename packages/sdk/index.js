import { createHash, randomUUID } from 'node:crypto';

export class SMSError extends Error {
  constructor(message, { provider, status, code, retryAfter, uncertain = false } = {}) {
    super(message); this.name = 'SMSError';
    Object.assign(this, { provider, status, code, retryAfter, uncertain });
  }
}

const endpoints = { local: 'http://127.0.0.1:18257', twilio: 'https://api.twilio.com', vonage: 'https://rest.nexmo.com', ovh: 'https://eu.api.ovh.com/1.0' };
const gsm = new Set([..."@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞÆæßÉ !\"#¤%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà\f^{}\\[~]|€"]);

export function createSMS(options = {}) {
  options = { ...options };
  const provider = options.driver || 'local';
  if (!Object.hasOwn(endpoints, provider)) throw new TypeError(`Unsupported SMS driver: ${provider}`);
  const base = (options.baseURL || endpoints[provider]).replace(/\/$/, '');
  const endpoint = new URL(base);
  if (!['http:', 'https:'].includes(endpoint.protocol) || endpoint.username || endpoint.password || endpoint.search || endpoint.hash) throw new TypeError('baseURL must be an HTTP(S) endpoint without credentials, query or fragment');
  const required = { local: [], twilio: ['accountSid', 'authToken'], vonage: ['apiKey', 'apiSecret'], ovh: ['appKey', 'appSecret', 'consumerKey', 'service'] };
  for (const key of required[provider]) if (!options[key]) throw new TypeError(`${provider} requires ${key}`);
  const timeout = options.timeout ?? 10000;
  if (!Number.isFinite(timeout) || timeout < 1 || timeout > 60000) throw new TypeError('timeout must be 1–60000 milliseconds');

  async function request(url, init, signal) {
    let response;
    try {
      response = await fetch(url, { ...init, redirect: 'error', signal: AbortSignal.any([AbortSignal.timeout(timeout), ...(signal ? [signal] : [])]) });
    } catch (cause) { throw new SMSError(`SMS request failed: ${cause.message}`, { provider, uncertain: init.method === 'POST' }); }
    let text = '';
    try {
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let bytes = 0;
      for (;;) {
        const chunk = await reader.read();
        if (chunk.done) break;
        bytes += chunk.value.byteLength;
        if (bytes > 1048576) { await reader.cancel(); throw new Error('response exceeds 1 MiB'); }
        text += decoder.decode(chunk.value, { stream: true });
      }
      text += decoder.decode();
    } catch (cause) { throw new SMSError(`Unable to read SMS response: ${cause.message}`, { provider, status: response.status, uncertain: init.method === 'POST' }); }
    let data;
    try { data = JSON.parse(text); }
    catch { throw new SMSError('Provider returned an invalid JSON response', { provider, status: response.status, uncertain: init.method === 'POST' }); }
    if (init.method === 'POST' && (!data || typeof data !== 'object' || Array.isArray(data))) throw new SMSError('Unexpected SMS response shape', { provider, status: response.status, uncertain: response.ok });
    if (!response.ok) throw new SMSError(data.error || data.message || `SMS request returned HTTP ${response.status}`, { provider, status: response.status, code: data.code, retryAfter: response.headers.get('retry-after') || undefined, uncertain: response.status >= 500 });
    return data;
  }

  return {
    async send(input) {
      if (!/^\+[1-9][0-9]{6,14}$/.test(input.to || '') || !input.from?.trim() || !input.body?.trim()) throw new TypeError('to must be E.164-shaped; from and body are required');
      if ([...input.body].length > 4096) throw new TypeError('body exceeds 4096 characters');
      if (provider === 'local') {
        const mode = input.mode || options.mode;
        const data = await request(`${base}/api/v1/messages`, {
          method: 'POST', headers: { 'Content-Type': 'application/json', ...(options.token ? { Authorization: `Bearer ${options.token}` } : {}) },
          body: JSON.stringify({ to: input.to, from: input.from, body: input.body, inbox: options.inbox || 'local', run_id: input.runId || '', scenario_id: input.scenarioId || options.scenarioId || '', callback_url: input.callbackURL || '', ...(mode ? { mode } : {}), ...(mode === 'relay' ? { idempotency_key: input.idempotencyKey || randomUUID() } : {}) }),
        }, input.signal);
        if (typeof data.id !== 'string' || typeof data.status !== 'string') throw new SMSError('TextDock returned an incomplete message result', { provider, uncertain: true });
        return { id: data.id, status: data.status, provider };
      }
      if (provider === 'twilio') {
        const body = new URLSearchParams({ To: input.to, From: input.from, Body: input.body });
        if (input.callbackURL) body.set('StatusCallback', input.callbackURL);
        const data = await request(`${base}/2010-04-01/Accounts/${encodeURIComponent(options.accountSid)}/Messages.json`, {
          method: 'POST', headers: { Authorization: `Basic ${Buffer.from(`${options.accountSid}:${options.authToken}`).toString('base64')}`, 'Content-Type': 'application/x-www-form-urlencoded' }, body: body.toString(),
        }, input.signal);
        if (typeof data.sid !== 'string' || !data.sid) throw new SMSError('Twilio response omitted the message SID', { provider, uncertain: true });
        return { id: data.sid, status: data.status || 'accepted', provider };
      }
      if (provider === 'vonage') {
        const data = await request(`${base}/sms/json`, {
          method: 'POST', headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ api_key: options.apiKey, api_secret: options.apiSecret, to: input.to.slice(1), from: input.from, text: input.body, type: [...input.body].every(char => gsm.has(char)) ? 'text' : 'unicode', ...(input.callbackURL ? { callback: input.callbackURL, 'status-report-req': true } : {}) }),
        }, input.signal);
        const message = data.messages?.[0];
        if (!message) throw new SMSError('Vonage response omitted the message result', { provider, uncertain: true });
        if (message.status !== '0') throw new SMSError(message['error-text'] || 'Vonage rejected the message', { provider, code: message.status });
        if (!message['message-id']) throw new SMSError('Vonage response omitted the message ID', { provider, uncertain: true });
        return { id: message['message-id'], status: 'accepted', provider };
      }
      if (input.callbackURL) throw new TypeError('OVH callbacks must be configured on the SMS service; per-message callbackURL is unsupported');
      const timestamp = await request(`${base}/auth/time`, { method: 'GET' }, input.signal);
      if (!Number.isSafeInteger(timestamp)) throw new SMSError('OVH returned an invalid server timestamp', { provider });
      const url = `${base}/sms/${encodeURIComponent(options.service)}/jobs`;
      const body = JSON.stringify({ receivers: [input.to], sender: input.from, message: input.body, priority: 'high', noStopClause: false });
      const signature = '$1$' + createHash('sha1').update([options.appSecret, options.consumerKey, 'POST', url, body, timestamp].join('+')).digest('hex');
      const data = await request(url, { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Ovh-Application': options.appKey, 'X-Ovh-Consumer': options.consumerKey, 'X-Ovh-Timestamp': String(timestamp), 'X-Ovh-Signature': signature }, body }, input.signal);
      if (!Array.isArray(data.ids)) throw new SMSError('OVH response omitted job IDs', { provider, uncertain: true });
      if (!data.ids.length) throw new SMSError('OVH did not accept a message job', { provider, code: 'no_job_id' });
      return { id: String(data.ids[0]), status: 'accepted', provider };
    },
  };
}

export function fromEnv(env = process.env) {
  return createSMS({
    driver: env.SMS_DRIVER || 'local', baseURL: env.SMS_ENDPOINT || ((env.SMS_DRIVER || 'local') === 'local' ? env.TEXTDOCK_URL : undefined),
    token: env.TEXTDOCK_TOKEN, inbox: env.TEXTDOCK_INBOX, scenarioId: env.TEXTDOCK_SCENARIO,
    mode: env.TEXTDOCK_MODE,
    accountSid: env.TWILIO_ACCOUNT_SID, authToken: env.TWILIO_AUTH_TOKEN,
    apiKey: env.VONAGE_API_KEY, apiSecret: env.VONAGE_API_SECRET,
    appKey: env.OVH_APP_KEY, appSecret: env.OVH_APP_SECRET, consumerKey: env.OVH_CONSUMER_KEY, service: env.OVH_SMS_SERVICE,
  });
}
