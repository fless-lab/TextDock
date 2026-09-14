/** Node/test-runner helper. Application code still generates and verifies OTPs. */
export async function waitForCode({
  url = 'http://127.0.0.1:18257', token = '', inbox = 'local',
  to, runId, timeout = 30, signal,
}) {
  if (!to || !runId) throw new Error('to and a unique runId are required');
  if (!Number.isInteger(timeout) || timeout < 0 || timeout > 30) throw new Error('timeout must be 0–30 seconds');
  const query = new URLSearchParams({ inbox, to, run_id: runId, timeout: String(timeout) });
  const signals = [AbortSignal.timeout((timeout + 5) * 1000)];
  if (signal) signals.push(signal);
  const response = await fetch(`${url.replace(/\/$/, '')}/api/v1/otp?${query}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    signal: AbortSignal.any(signals),
    redirect: 'error',
  });
  const data = await response.json();
  if (!response.ok) throw new Error(`TextDock ${response.status}: ${data.error}`);
  return data.code;
}
