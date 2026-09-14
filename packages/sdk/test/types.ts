import { createSMS, fromEnv, SMSError, type SMSResult } from '../index.js';

const client = createSMS({ driver: 'local', inbox: 'local' });
const result: Promise<SMSResult> = client.send({ to: '+33612345678', from: 'Acme', body: 'Code 123456' });
void result;
fromEnv({ SMS_DRIVER: 'twilio', TWILIO_ACCOUNT_SID: 'ACtest', TWILIO_AUTH_TOKEN: 'test' });
const error = new SMSError('failed', { provider: 'twilio', status: 503, uncertain: true });
const uncertain: boolean = error.uncertain;
void uncertain;
// @ts-expect-error unknown drivers are rejected by the public type contract
createSMS({ driver: 'unknown' });
// @ts-expect-error sending requires a body
client.send({ to: '+33612345678', from: 'Acme' });
