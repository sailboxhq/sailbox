const API_BASE = "";

type RequestOptions = {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
};

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

  private async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
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
      if (res.status === 401 && typeof window !== "undefined" && !path.includes("/auth/")) {
        localStorage.removeItem("sailbox_token");
        localStorage.removeItem("sailbox_refresh");
        this.token = null;
        if (this.onUnauthorized) {
          this.onUnauthorized();
        } else {
          // Fallback: direct redirect if callback not registered yet
          window.location.href = "/auth/login";
        }
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

  post<T>(path: string, body?: unknown) {
    return this.request<T>(path, { method: "POST", body });
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
