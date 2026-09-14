const form = document.querySelector('#verify');
const input = document.querySelector('#code');
const status = document.querySelector('#status');
const listen = document.querySelector('#listen');
document.querySelector('#format').textContent = `Your code is 482193.\n\n@${location.hostname} #482193`;
let controller;

listen.addEventListener('click', async () => {
  if (!window.isSecureContext || !('OTPCredential' in window)) {
    status.textContent = 'WebOTP is unavailable. Use a supported browser on HTTPS, or enter the code manually.';
    return;
  }
  controller?.abort();
  controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 60_000);
  listen.disabled = true;
  status.textContent = 'Waiting for a real SMS and browser consent…';
  try {
    const credential = await navigator.credentials.get({ otp: { transport: ['sms'] }, signal: controller.signal });
    if (credential) {
      input.value = credential.code;
      status.textContent = 'Code received. Continue to verify it with your application backend.';
    }
  } catch (error) {
    status.textContent = error.name === 'AbortError' ? 'Listener stopped. Enter the code manually or try again.' : `WebOTP unavailable: ${error.message}`;
  } finally {
    clearTimeout(timer);
    listen.disabled = false;
  }
});

form.addEventListener('submit', event => {
  event.preventDefault();
  controller?.abort();
  status.textContent = 'Example only: submit the entered code to your application backend for verification.';
});
window.addEventListener('pagehide', () => controller?.abort());
