import { useState, type FormEvent } from "react";
import { api } from "../api";
import { copyText } from "../clipboard";

export function OTPFormat() {
  const [format, setFormat] = useState("webotp");
  const [body, setBody] = useState("");
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);
  async function generate(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError("");
    setCopied(false);
    try {
      setBody(
        (
          await api<{ body: string }>("/otp/format", {
            method: "POST",
            body: JSON.stringify({
              ...Object.fromEntries(new FormData(e.currentTarget)),
              format,
            }),
          })
        ).body,
      );
    } catch (e) {
      setError((e as Error).message);
    }
  }
  return (
    <section>
      <h3>Native OTP message format</h3>
      <p className="modal-description">
        Format an application-generated code. WebOTP needs your application’s
        HTTPS hostname; Android Retriever needs its signing-specific app hash.
        Formatting alone does not verify device autofill.{" "}
        <a
          href="https://github.com/fless-lab/TextDock/blob/main/docs/NATIVE-SAMPLES.md"
          target="_blank"
          rel="noreferrer"
        >
          Native sample apps and test steps
        </a>
      </p>
      <form
        onSubmit={(e) => {
          void generate(e);
        }}
      >
        <label>
          Platform
          <select
            value={format}
            onChange={(e) => {
              setFormat(e.target.value);
              setBody("");
            }}
          >
            <option value="webotp">WebOTP</option>
            <option value="android">Android SMS Retriever</option>
          </select>
        </label>
        <label>
          Application code
          <input name="code" defaultValue="482193" maxLength={10} required />
        </label>
        {format === "webotp" ? (
          <label>
            Application hostname
            <input name="domain" placeholder="login.example.test" required />
          </label>
        ) : (
          <label>
            Android app hash
            <input
              name="app_hash"
              placeholder="11-character app hash"
              required
              maxLength={11}
            />
          </label>
        )}
        <button className="secondary">Format message</button>
      </form>
      {body && (
        <>
          <pre className="code-block">{body}</pre>
          <button
            className="secondary"
            onClick={async () => setCopied(await copyText(body))}
          >
            {copied ? "Copied" : "Copy formatted message"}
          </button>
        </>
      )}
      {error && (
        <p role="alert" className="error-text">
          {error}
        </p>
      )}
    </section>
  );
}
