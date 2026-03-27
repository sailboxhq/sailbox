import { api } from "./api";

const TOKEN_KEY = "sailbox_token";
const REFRESH_KEY = "sailbox_refresh";

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(TOKEN_KEY);
}

export function setTokens(accessToken: string, refreshToken: string) {
  localStorage.setItem(TOKEN_KEY, accessToken);
  localStorage.setItem(REFRESH_KEY, refreshToken);
  api.setToken(accessToken);
}

export function clearTokens() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_KEY);
  api.setToken(null);
}

// Initialize auth — call this once per page. Synchronous, safe to call before API.
export function initAuth() {
  const token = getToken();
  if (token) {
    api.setToken(token);
    return true;
  }
  return false;
}

// Register the 401 redirect handler (called from layout)
export function setupAuthRedirect(redirectFn: () => void) {
  api.setUnauthorizedHandler(redirectFn);
}

export type LoginResult = {
  user?: any;
  access_token?: string;
  refresh_token?: string;
  requires_2fa?: boolean;
};

export async function login(email: string, password: string, twoFACode?: string) {
  const body: Record<string, string> = { email, password };
  if (twoFACode) {
    body.two_fa_code = twoFACode;
  }
  const res = await api.post<LoginResult>("/api/v1/auth/login", body);
  if (res.access_token && res.refresh_token) {
    setTokens(res.access_token, res.refresh_token);
  }
  return res;
}

export async function register(
  email: string,
  password: string,
  displayName: string,
  orgName: string,
) {
  const res = await api.post<{
    user: any;
    access_token: string;
    refresh_token: string;
  }>("/api/v1/auth/register", {
    email,
    password,
    display_name: displayName,
    org_name: orgName,
  });
  setTokens(res.access_token, res.refresh_token);
  return res;
}
