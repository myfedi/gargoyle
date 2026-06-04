import { getApiBaseUrl, trimTrailingSlash } from "@/lib/config";


export type ApiErrorBody = {
  error?: string;
  message?: string;
};

export class ApiError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, body: unknown) {
    super(readApiErrorMessage(status, body));
    this.name = "ApiError";
    this.status = status;
    this.body = body;
  }
}

export type ApiClientOptions = {
  baseUrl?: string;
  accessToken?: string;
};

export type ApiRequestOptions = RequestInit & {
  accessToken?: string;
};

const inFlightGetRequests = new Map<string, Promise<unknown>>();

export class ApiClient {
  private readonly baseUrl: string;
  private readonly accessToken?: string;

  constructor(options: ApiClientOptions = {}) {
    this.baseUrl = trimTrailingSlash(options.baseUrl ?? getApiBaseUrl());
    this.accessToken = options.accessToken;
  }

  async request<T>(path: string, options: ApiRequestOptions = {}): Promise<T> {
    const headers = new Headers(options.headers);
    headers.set("Accept", "application/json");

    if (options.body && !headers.has("Content-Type") && !(options.body instanceof FormData)) {
      headers.set("Content-Type", "application/json");
    }

    const token = options.accessToken ?? this.accessToken;
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }

    const method = (options.method ?? "GET").toUpperCase();
    const url = `${this.baseUrl}${normalizePath(path)}`;
    if (method === "GET") {
      const key = `${token ?? ""} ${url}`;
      const existing = inFlightGetRequests.get(key);
      if (existing) {
        return existing as Promise<T>;
      }
      const request = this.fetchJSON<T>(url, { ...options, headers, credentials: "same-origin" });
      inFlightGetRequests.set(key, request);
      try {
        return await request;
      } finally {
        inFlightGetRequests.delete(key);
      }
    }

    return this.fetchJSON<T>(url, { ...options, headers, credentials: "same-origin" });
  }

  private async fetchJSON<T>(url: string, options: RequestInit): Promise<T> {
    const response = await fetch(url, options);
    const body = await readResponseBody(response);
    if (!response.ok) {
      throw new ApiError(response.status, body);
    }
    return body as T;
  }
}

export const apiClient = new ApiClient();

function normalizePath(path: string) {
  return path.startsWith("/") ? path : `/${path}`;
}

async function readResponseBody(response: Response): Promise<unknown> {
  const contentType = response.headers.get("Content-Type") ?? "";
  if (contentType.includes("application/json")) {
    return response.json();
  }
  return response.text();
}

function readApiErrorMessage(status: number, body: unknown) {
  if (body && typeof body === "object") {
    const typedBody = body as ApiErrorBody;
    return typedBody.message ?? typedBody.error ?? `Request failed with status ${status}`;
  }
  return `Request failed with status ${status}`;
}
