/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_GARGOYLE_API_BASE_URL?: string;
  readonly VITE_GARGOYLE_OAUTH_CLIENT_ID?: string;
  readonly VITE_GARGOYLE_OAUTH_AUTHORIZE_URL?: string;
  readonly VITE_GARGOYLE_OAUTH_TOKEN_URL?: string;
  readonly VITE_GARGOYLE_OAUTH_REVOKE_URL?: string;
  readonly VITE_GARGOYLE_OAUTH_REDIRECT_URI?: string;
  readonly VITE_GARGOYLE_OAUTH_SCOPES?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
