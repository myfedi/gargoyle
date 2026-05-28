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

  const baseUrl = getApiBaseUrl().replace(/\/+$/, "");

  return {
    clientId,
    authorizationEndpoint: import.meta.env.VITE_GARGOYLE_OAUTH_AUTHORIZE_URL ?? `${baseUrl}/oauth/authorize`,
    tokenEndpoint: import.meta.env.VITE_GARGOYLE_OAUTH_TOKEN_URL ?? `${baseUrl}/oauth/token`,
    redirectUri: import.meta.env.VITE_GARGOYLE_OAUTH_REDIRECT_URI ?? `${window.location.origin}/oauth/callback`,
    scopes: parseScopes(import.meta.env.VITE_GARGOYLE_OAUTH_SCOPES ?? "read write follow"),
  };
}

function parseScopes(value: string) {
  return value
    .split(/[\s,]+/)
    .map((scope) => scope.trim())
    .filter(Boolean);
}
