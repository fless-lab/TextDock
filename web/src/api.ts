export interface Message {
  id: string;
  to: string;
  from: string;
  body: string;
  run_id: string;
  source: string;
  status: string;
  created_at: string;
  analysis: {
    encoding: string;
    units: number;
    characters: number;
    segments: number;
    otp?: string;
  };
}

export interface Info {
  name: string;
  version: string;
  mode: string;
  auth_enabled: boolean;
}

export class APIError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

export async function api<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers);
  const token = sessionStorage.getItem("textdock-token");
  if (token) headers.set("Authorization", `Bearer ${token}`);
  if (options.body) headers.set("Content-Type", "application/json");
  const response = await fetch(`/api/v1${path}`, { ...options, headers });
  if (!response.ok) {
    const body = await response
      .json()
      .catch(() => ({ error: response.statusText }));
    throw new APIError(response.status, body.error || "Request failed");
  }
  return response.status === 204 ? (undefined as T) : response.json();
}
