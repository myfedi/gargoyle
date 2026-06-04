import type { OAuthClientConfig } from "@/lib/config";
import type { TokenResponse } from "@/types/auth";

const verifierStorageKey = "gargoyle.oauth.verifier";
const stateStorageKey = "gargoyle.oauth.state";

export async function createAuthorizationUrl(config: OAuthClientConfig) {
  const verifier = createRandomToken();
  const state = createRandomToken();
  const challenge = await createPkceChallenge(verifier);

  sessionStorage.setItem(verifierStorageKey, verifier);
  sessionStorage.setItem(stateStorageKey, state);

  const url = new URL(config.authorizationEndpoint, globalThis.location.origin);
  url.searchParams.set("response_type", "code");
  url.searchParams.set("client_id", config.clientId);
  url.searchParams.set("redirect_uri", config.redirectUri);
  url.searchParams.set("scope", config.scopes.join(" "));
  url.searchParams.set("state", state);
  url.searchParams.set("code_challenge", challenge);
  url.searchParams.set("code_challenge_method", "S256");

  return url;
}

export async function exchangeAuthorizationCode(config: OAuthClientConfig, code: string) {
  const verifier = readStoredVerifier();
  if (!verifier) {
    throw new Error("Missing OAuth verifier. Start authorization again.");
  }

  const body = {
    grant_type: "authorization_code",
    code,
    client_id: config.clientId,
    redirect_uri: config.redirectUri,
    code_verifier: verifier,
  };

  const tokenUrl = new URL(config.tokenEndpoint, globalThis.location.origin);

  const response = await fetch(tokenUrl, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
    credentials: "same-origin",
  });

  const payload = await readTokenResponse(response);
  if (!response.ok) {
    throw new Error(readOAuthError(payload, response.status));
  }

  return payload as TokenResponse;
}

export function validateOAuthState(receivedState: string | null) {
  const expectedState = sessionStorage.getItem(stateStorageKey);
  return Boolean(receivedState && expectedState && receivedState === expectedState);
}

export function clearOAuthTransaction() {
  sessionStorage.removeItem(verifierStorageKey);
  sessionStorage.removeItem(stateStorageKey);
}

function readStoredVerifier() {
  return sessionStorage.getItem(verifierStorageKey);
}

function createRandomToken() {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return base64UrlEncode(bytes);
}

async function createPkceChallenge(verifier: string) {
  const bytes = new TextEncoder().encode(verifier);
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return base64UrlEncode(new Uint8Array(digest));
}

function base64UrlEncode(bytes: Uint8Array) {
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCodePoint(byte);
  });
  return trimBase64UrlPadding(btoa(binary).replaceAll("+", "-").replaceAll("/", "_"));
}

function trimBase64UrlPadding(value: string) {
  // Standard base64 may end with '=' padding, but PKCE code challenges use
  // unpadded base64url, so remove only trailing padding characters.
  const paddingCode = "=".charCodeAt(0);
  let end = value.length;
  while (end > 0 && value.charCodeAt(end - 1) === paddingCode) {
    end -= 1;
  }
  return end === value.length ? value : value.slice(0, end);
}

async function readTokenResponse(response: Response): Promise<unknown> {
  const contentType = response.headers.get("Content-Type") ?? "";
  if (contentType.includes("application/json")) {
    return response.json().catch(() => null);
  }
  return response.text().catch(() => null);
}

function readOAuthError(payload: unknown, status: number) {
  if (payload && typeof payload === "object") {
    const error = payload as { error?: string; error_description?: string; message?: string };
    return error.error_description ?? error.message ?? error.error ?? `OAuth token exchange failed with status ${status}`;
  }
  if (typeof payload === "string" && payload.trim()) {
    return payload;
  }
  return `OAuth token exchange failed with status ${status}`;
}
