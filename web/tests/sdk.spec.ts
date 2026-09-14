import { expect, test } from '@playwright/test';
import { createSMS } from '../../packages/sdk/index.js';

test('common SDK sends through all local provider adapters', async ({ request }) => {
  const baseURL = 'http://127.0.0.1:18258';
  const token = 'textdock-e2e-token-only';
  const clients = [
    createSMS({ baseURL, token }),
    createSMS({ driver: 'twilio', baseURL, accountSid: 'AC'+'0'.repeat(32), authToken: token }),
    createSMS({ driver: 'vonage', baseURL, apiKey: 'fake-key', apiSecret: token }),
    createSMS({ driver: 'ovh', baseURL: baseURL+'/1.0', appKey: 'fake-key', appSecret: 'fake-secret', consumerKey: token, service: 'local-service' }),
  ];
  for (const [index, client] of clients.entries()) {
    const result = await client.send({ to: '+33612345678', from: 'Acme', body: `Common SDK ${index} code 918273` });
    expect(result.id).toBeTruthy();
  }
  const response = await request.get('/api/v1/messages?q=Common%20SDK', { headers: { Authorization: `Bearer ${token}` } });
  const { messages } = await response.json();
  expect(new Set(messages.map((m: { source: string }) => m.source))).toEqual(new Set(['api', 'twilio', 'vonage', 'ovh']));
});
