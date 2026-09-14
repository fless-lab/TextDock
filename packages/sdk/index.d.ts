export type Driver = 'local' | 'twilio' | 'vonage' | 'ovh';
export interface SMSOptions {
  mode?: 'capture' | 'simulate' | 'relay';
  driver?: Driver; baseURL?: string; timeout?: number;
  token?: string; inbox?: string; scenarioId?: string;
  accountSid?: string; authToken?: string;
  apiKey?: string; apiSecret?: string;
  appKey?: string; appSecret?: string; consumerKey?: string; service?: string;
}
export interface SMSInput {
  mode?: 'capture' | 'simulate' | 'relay'; idempotencyKey?: string;
  to: string; from: string; body: string;
  runId?: string; scenarioId?: string; callbackURL?: string; signal?: AbortSignal;
}
export interface SMSResult { id: string; status: string; provider: Driver }
export interface SMSClient { send(input: SMSInput): Promise<SMSResult> }
export class SMSError extends Error {
  constructor(message: string, options?: { provider?: Driver; status?: number; code?: string | number; retryAfter?: string; uncertain?: boolean });
  provider?: Driver; status?: number; code?: string | number;
  retryAfter?: string; uncertain: boolean;
}
export function createSMS(options?: SMSOptions): SMSClient;
export function fromEnv(env?: Record<string, string | undefined>): SMSClient;
