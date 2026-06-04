const defaultApiBaseUrl = "/api";

export type OAuthClientConfig = {
  clientId: string;
  authorizationEndpoint: string;
  tokenEndpoint: string;
  redirectUri: string;
  scopes: string[];
};

export function getApiBaseUrl() {
  return import.meta.env.VITE_GARGOYLE_API_BASE_URL ?? defaultApiBaseUrl;
}

export function getOAuthConfig(): OAuthClientConfig | null {
  const clientId = import.meta.env.VITE_GARGOYLE_OAUTH_CLIENT_ID;
  if (!clientId) {
    return null;
  }

  const baseUrl = trimTrailingSlash(getApiBaseUrl());

  return {
    clientId,
    authorizationEndpoint: import.meta.env.VITE_GARGOYLE_OAUTH_AUTHORIZE_URL ?? `${baseUrl}/oauth/authorize`,
    tokenEndpoint: import.meta.env.VITE_GARGOYLE_OAUTH_TOKEN_URL ?? `${baseUrl}/oauth/token`,
    redirectUri: import.meta.env.VITE_GARGOYLE_OAUTH_REDIRECT_URI ?? `${globalThis.location.origin}/oauth/callback`,
    scopes: parseScopes(import.meta.env.VITE_GARGOYLE_OAUTH_SCOPES ?? "read write follow"),
  };
}

export function trimTrailingSlash(value: string) {
  const slashCode = "/".charCodeAt(0);
  let end = value.length;
  while (end > 0 && value.charCodeAt(end - 1) === slashCode) {
    end -= 1;
  }
  return end === value.length ? value : value.slice(0, end);
}

function parseScopes(value: string) {
  return value
    .split(/[\s,]+/)
    .map((scope) => scope.trim())
    .filter(Boolean);
}
