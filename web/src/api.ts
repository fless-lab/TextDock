export interface Message {
  id: string;
  inbox: string;
  mode: "capture" | "simulate" | "relay";
  direction: "outbound" | "inbound";
  favorite: boolean;
  tags: string[];
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
    non_gsm?: string[];
  };
}

export interface Info {
  name: string;
  version: string;
  mode: string;
  auth_enabled: boolean;
  operator: boolean;
  session?: UserSession;
  api_key?: { permissions: string[] };
}

export interface User {
  id: string;
  username: string;
  name: string;
  disabled: boolean;
}
export interface Membership {
  project_id: string;
  user_id: string;
  username: string;
  name: string;
  role: "viewer" | "member" | "admin";
}
export interface UserSession {
  id: string;
  user: User;
  memberships: Membership[];
  created_at: string;
  expires_at: string;
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
