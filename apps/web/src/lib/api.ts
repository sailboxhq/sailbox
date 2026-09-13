const API_BASE = "";

type RequestOptions = {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
};

// Endpoints where a 401 is the answer rather than an expired session: the
// credential exchanges themselves. Everything else — including authenticated
// /auth/* routes such as /auth/me, which the dashboard loader calls on every
// page load — must be allowed to refresh, or a short-lived access token logs
// the user out instead of renewing.
const NO_REFRESH_PATHS = ["/api/v1/auth/login", "/api/v1/auth/register", "/api/v1/auth/refresh"];

// Treats a token as stale slightly before it actually expires, so a connection
// opened right on the boundary is not rejected. An undecodable token is left
// alone — guessing would cause a refresh on every call.
const EXPIRY_SKEW_SECONDS = 30;

function isExpiringSoon(token: string): boolean {
  try {
    const payload = token.split(".")[1];
    if (!payload) return false;
    const json = atob(payload.replace(/-/g, "+").replace(/_/g, "/"));
    const exp = JSON.parse(json).exp as number | undefined;
    if (typeof exp !== "number") return false;
    return exp - EXPIRY_SKEW_SECONDS <= Date.now() / 1000;
  } catch {
    return false;
  }
}

function canRefresh(path: string): boolean {
  return !NO_REFRESH_PATHS.some((p) => path.startsWith(p));
}

// Custom error for 401 — components should catch and redirect via router
export class UnauthorizedError extends Error {
  constructor() {
    super("Unauthorized");
    this.name = "UnauthorizedError";
  }
}

class ApiClient {
  private baseUrl: string;
  private token: string | null = null;
  private onUnauthorized: (() => void) | null = null;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  setToken(token: string | null) {
    this.token = token;
  }

  // Called from the auth provider to register a redirect callback
  setUnauthorizedHandler(handler: () => void) {
    this.onUnauthorized = handler;
  }

  // Exchanges the stored refresh token for a new access token. Concurrent
  // callers share one in-flight refresh so a burst of 401s triggers a single
  // round trip.
  private refreshInFlight: Promise<boolean> | null = null;

  private async refreshAccessToken(): Promise<boolean> {
    if (this.refreshInFlight) return this.refreshInFlight;

    this.refreshInFlight = (async () => {
      const refreshToken =
        typeof window !== "undefined" ? localStorage.getItem("sailbox_refresh") : null;
      if (!refreshToken) return false;

      try {
        const res = await fetch(`${this.baseUrl}/api/v1/auth/refresh`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refresh_token: refreshToken }),
        });
        if (!res.ok) return false;

        const data = (await res.json()) as {
          access_token?: string;
          refresh_token?: string;
        };
        if (!data.access_token || !data.refresh_token) return false;

        localStorage.setItem("sailbox_token", data.access_token);
        localStorage.setItem("sailbox_refresh", data.refresh_token);
        this.token = data.access_token;
        return true;
      } catch {
        return false;
      } finally {
        this.refreshInFlight = null;
      }
    })();

    return this.refreshInFlight;
  }

  // Streams (EventSource / WebSocket) carry the token in the URL and never get a
  // 401 they can retry, so they ask for a usable token up front instead. With a
  // short access-token lifetime, a reconnect after expiry would otherwise loop
  // on 401 until some unrelated REST call happened to refresh.
  async ensureFreshToken(): Promise<string | null> {
    const token = this.token ?? localStorage.getItem("sailbox_token");
    if (token && !isExpiringSoon(token)) return token;
    if (await this.refreshAccessToken()) return this.token;
    return token; // let the connection fail; the normal 401 path signs out
  }

  private signOut() {
    localStorage.removeItem("sailbox_token");
    localStorage.removeItem("sailbox_refresh");
    this.token = null;
    if (this.onUnauthorized) {
      this.onUnauthorized();
    } else {
      // Fallback: direct redirect if callback not registered yet
      window.location.href = "/auth/login";
    }
  }

  private async request<T>(
    path: string,
    options: RequestOptions = {},
    isRetry = false,
  ): Promise<T> {
    const { method = "GET", body, headers = {} } = options;

    const requestHeaders: Record<string, string> = {
      "Content-Type": "application/json",
      ...headers,
    };

    if (this.token) {
      requestHeaders.Authorization = `Bearer ${this.token}`;
    }

    const res = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers: requestHeaders,
      body: body ? JSON.stringify(body) : undefined,
    });

    if (!res.ok) {
      if (res.status === 401 && typeof window !== "undefined" && canRefresh(path)) {
        // Access tokens are short-lived by design. Try the refresh token once
        // before treating a 401 as "signed out" — only a refresh that also
        // fails (expired, or revoked by a password change) ends the session.
        if (!isRetry && (await this.refreshAccessToken())) {
          return this.request<T>(path, options, true);
        }
        this.signOut();
        throw new UnauthorizedError();
      }
      const error = await res.json().catch(() => ({ detail: "Request failed" }));
      throw error;
    }

    if (res.status === 204) {
      return undefined as T;
    }

    return res.json();
  }

  get<T>(path: string) {
    return this.request<T>(path);
  }

  post<T>(path: string, body?: unknown, headers?: Record<string, string>) {
    return this.request<T>(path, { method: "POST", body, headers });
  }

  put<T>(path: string, body?: unknown) {
    return this.request<T>(path, { method: "PUT", body });
  }

  patch<T>(path: string, body?: unknown) {
    return this.request<T>(path, { method: "PATCH", body });
  }

  delete<T>(path: string) {
    return this.request<T>(path, { method: "DELETE" });
  }
}

export const api = new ApiClient(API_BASE);
