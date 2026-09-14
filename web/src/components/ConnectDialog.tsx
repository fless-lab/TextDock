import { useEffect, useState, type FormEvent } from "react";
import { Copy, RefreshCw, Smartphone } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { api } from "../api";
import { copyText } from "../clipboard";
import { Modal } from "./Modal";

export interface Device {
  id: string;
  name: string;
  scope: { to: string; run_id: string };
  created_at: string;
  expires_at: string;
  revoked: boolean;
}
interface Pair {
  code: string;
  expires_at: string;
  scope: Device["scope"];
}
interface Network {
  lan_enabled: boolean;
  urls: string[];
}

export function ConnectDialog({ close }: { close: () => void }) {
  const [devices, setDevices] = useState<Device[]>([]);
  const [network, setNetwork] = useState<Network>();
  const [base, setBase] = useState(location.origin);
  const [pair, setPair] = useState<Pair>();
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [now, setNow] = useState(Date.now());
  async function refresh() {
    try {
      setDevices((await api<{ devices: Device[] }>("/devices")).devices);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  useEffect(() => {
    void refresh();
    void api<Network>("/network")
      .then((data) => {
        setNetwork(data);
        setBase(data.urls[0] || location.origin);
      })
      .catch((e) => setError(e.message));
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);
  const remaining = pair
    ? Math.max(0, Math.ceil((Date.parse(pair.expires_at) - now) / 1000))
    : 0;
  const link = pair
    ? `${base.replace(/\/$/, "")}/phone#pair=${encodeURIComponent(pair.code)}`
    : "";
  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const values = new FormData(e.currentTarget);
    try {
      const url = new URL(base);
      if (
        !["http:", "https:"].includes(url.protocol) ||
        url.username ||
        url.password ||
        url.search ||
        url.hash ||
        url.pathname !== "/"
      )
        throw new Error(
          "Enter an HTTP(S) origin, without a path or credentials.",
        );
      setPair(
        await api<Pair>("/pairings", {
          method: "POST",
          body: JSON.stringify({
            to: values.get("to"),
            run_id: values.get("run_id"),
          }),
        }),
      );
      setNow(Date.now());
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function revoke(id: string) {
    setBusy(true);
    try {
      await api(`/devices/${id}`, { method: "DELETE" });
      await refresh();
      setNotice("Device access revoked.");
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Modal title="Connect a phone" close={close}>
      <p className="modal-description">
        Scan a QR code to open a read-only inbox for one recipient. The link can
        be used once and expires after two minutes.
      </p>
      {network && !network.lan_enabled && (
        <p className="callout">
          This server is listening locally. For a physical phone, restart with{" "}
          <code>--listen 0.0.0.0:18257</code> and a <code>TEXTDOCK_TOKEN</code>{" "}
          of at least 16 characters.
        </p>
      )}
      <form
        onSubmit={(e) => {
          void create(e);
        }}
      >
        <label>
          Server address
          <input
            type="url"
            value={base}
            onChange={(e) => {
              setBase(e.target.value);
              setPair(undefined);
            }}
            list="network-addresses"
            required
          />
        </label>
        <datalist id="network-addresses">
          {network?.urls.map((url) => (
            <option key={url} value={url} />
          ))}
        </datalist>
        <div className="form-row">
          <label>
            Recipient
            <input
              name="to"
              type="tel"
              defaultValue="+33612345678"
              pattern="\+[1-9][0-9]{6,14}"
              required
              onChange={() => setPair(undefined)}
            />
          </label>
          <label>
            Test run <span className="optional">optional</span>
            <input
              name="run_id"
              maxLength={128}
              placeholder="All runs for this recipient"
              onChange={() => setPair(undefined)}
            />
          </label>
        </div>
        <button className="primary" disabled={busy}>
          <Smartphone size={15} />
          {busy ? "Working…" : "Create pairing code"}
        </button>
      </form>
      {pair && (
        <div className="pairing-result">
          {remaining > 0 ? (
            <>
              <div className="pairing-qr">
                <QRCodeSVG
                  value={link}
                  size={176}
                  marginSize={2}
                  title="Phone pairing QR code"
                />
              </div>
              <p>
                {pair.scope.to} · Expires in {remaining}s
              </p>
              <input
                aria-label="Pairing link"
                readOnly
                value={link}
                onFocus={(e) => e.currentTarget.select()}
              />
              <button
                className="secondary"
                onClick={async () =>
                  setNotice(
                    (await copyText(link))
                      ? "Link copied."
                      : "Select the link above to copy it.",
                  )
                }
              >
                <Copy size={14} />
                Copy link
              </button>
            </>
          ) : (
            <p>Pairing code expired. Create a new code above.</p>
          )}
        </div>
      )}
      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="modal-description">
          {notice}
        </p>
      )}
      <div className="devices-heading">
        <h3>Connected devices</h3>
        <button
          className="icon-button"
          aria-label="Refresh devices"
          onClick={() => {
            void refresh();
          }}
        >
          <RefreshCw size={15} />
        </button>
      </div>
      {!devices.length && (
        <p className="modal-description">No paired devices yet.</p>
      )}
      <ul className="device-list">
        {devices.map((device) => (
          <li key={device.id}>
            <div>
              <strong>{device.name}</strong>
              <small>
                {device.scope.to}
                {device.scope.run_id ? ` · ${device.scope.run_id}` : ""}
              </small>
            </div>
            {device.revoked ? (
              <span className="muted">Revoked</span>
            ) : Date.parse(device.expires_at) <= now ? (
              <span className="muted">Expired</span>
            ) : (
              <button
                className="text-button"
                disabled={busy}
                onClick={() => {
                  void revoke(device.id);
                }}
              >
                Revoke
              </button>
            )}
          </li>
        ))}
      </ul>
      <p className="modal-description">
        Use your computer’s LAN address on the same Wi-Fi. For Docker or HTTPS,
        set <code>TEXTDOCK_PUBLIC_URL</code> to the address reachable from the
        phone.
      </p>
    </Modal>
  );
}
