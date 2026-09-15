import { deviceAPI } from "./api";

export async function phoneWorker(): Promise<ServiceWorkerRegistration> {
  await navigator.serviceWorker.register("/phone-sw.js", { scope: "/phone" });
  return new Promise((resolve, reject) => {
    const timer = setTimeout(
      () =>
        reject(
          new Error(
            "The phone service worker did not become ready. Reload the page.",
          ),
        ),
      8000,
    );
    navigator.serviceWorker.ready.then(
      (reg) => {
        clearTimeout(timer);
        resolve(reg);
      },
      (error) => {
        clearTimeout(timer);
        reject(error);
      },
    );
  });
}
export async function bindNotifications(
  reg: ServiceWorkerRegistration,
  deviceId: string,
  generation: string,
  expiresAt = "",
) {
  const worker = reg.active;
  if (!worker) throw new Error("Phone service worker is not active");
  await new Promise<void>((resolve, reject) => {
    const channel = new MessageChannel();
    const timer = setTimeout(() => {
      channel.port1.close();
      reject(new Error("Unable to update notification session"));
    }, 3000);
    channel.port1.onmessage = (event) => {
      clearTimeout(timer);
      channel.port1.close();
      if (event.data?.ok) resolve();
      else reject(new Error("Unable to save notification session"));
    };
    worker.postMessage(
      {
        type: "textdock-bind",
        deviceId,
        generation,
        expiresAt: Date.parse(expiresAt) || 0,
      },
      [channel.port2],
    );
  });
}
export async function disableNotifications(token: string | null) {
  if ("serviceWorker" in navigator && window.isSecureContext) {
    const reg = await navigator.serviceWorker.getRegistration("/phone");
    if (reg) {
      await bindNotifications(reg, "", "").catch(() => {});
      if ("pushManager" in reg)
        await (await reg.pushManager.getSubscription())?.unsubscribe();
    }
  }
  if (token)
    await deviceAPI("/push/subscription", token, {
      method: "DELETE",
      signal: AbortSignal.timeout(3000),
    }).catch(() => {});
}
export function publicKeyBytes(value: string): Uint8Array<ArrayBuffer> {
  const decoded = atob(value.replace(/-/g, "+").replace(/_/g, "/"));
  return Uint8Array.from(decoded, (char) => char.charCodeAt(0));
}
export function samePublicKey(a: ArrayBuffer | null, b: Uint8Array): boolean {
  if (!a) return false;
  const bytes = new Uint8Array(a);
  return (
    bytes.length === b.length &&
    bytes.every((value, index) => value === b[index])
  );
}
