import { getApiBaseUrl } from "@/lib/config";


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

    if (options.body && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }

    const token = options.accessToken ?? this.accessToken;
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }

    const response = await fetch(`${this.baseUrl}${normalizePath(path)}`, {
      ...options,
      headers,
      credentials: "same-origin",
    });

    const body = await readResponseBody(response);
    if (!response.ok) {
      throw new ApiError(response.status, body);
    }

    return body as T;
  }
}

export const apiClient = new ApiClient();

function trimTrailingSlash(value: string) {
  return value.replace(/\/+$/, "");
}

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
