import type React from "react";
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";

import { authSessionKey, clearAuthSession, readAuthSession, writeAuthSession } from "@/lib/auth-storage";
import { getOAuthConfig } from "@/lib/config";
import { clearOAuthTransaction, createAuthorizationUrl, exchangeAuthorizationCode, revokeAccessToken, validateOAuthState } from "@/lib/oauth";
import type { AuthSession } from "@/types/auth";

type AuthStatus = "checking" | "authenticated" | "unauthenticated";

type AuthContextValue = {
  session: AuthSession | null;
  status: AuthStatus;
  error: string | null;
  signIn: () => Promise<void>;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);
const oauthExchangePromises = new Map<string, Promise<AuthSession>>();
const oauthReturnToKey = "gargoyle.oauth.return_to";

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<AuthSession | null>(() => readAuthSession());
  const [status, setStatus] = useState<AuthStatus>(() => (session ? "authenticated" : "checking"));
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function completeOAuthIfPresent() {
      const params = new URLSearchParams(globalThis.location.search);
      const code = params.get("code");
      const state = params.get("state");
      const oauthError = params.get("error_description") ?? params.get("error");

      if (oauthError) {
        setError(oauthError);
        setStatus(session ? "authenticated" : "unauthenticated");
        return;
      }

      if (!code) {
        setStatus(session ? "authenticated" : "unauthenticated");
        return;
      }

      if (!validateOAuthState(state)) {
        clearOAuthTransaction();
        setError("Sign-in could not be verified. Start again.");
        setStatus("unauthenticated");
        return;
      }

      const config = getOAuthConfig();
      if (!config) {
        setError("Sign-in is not configured.");
        setStatus("unauthenticated");
        return;
      }

      try {
        const nextSession = await getOAuthExchangePromise(code, async () => {
          const token = await exchangeAuthorizationCode(config, code);
          return {
            accessToken: token.access_token,
            tokenType: token.token_type,
            scope: token.scope,
            createdAt: token.created_at,
            expiresIn: token.expires_in,
            refreshToken: token.refresh_token,
          };
        });

        if (cancelled) {
          return;
        }

        writeAuthSession(nextSession);
        clearOAuthTransaction();
        setSession(nextSession);
        setError(null);
        setStatus("authenticated");
        const returnTo = globalThis.sessionStorage.getItem(oauthReturnToKey);
        globalThis.sessionStorage.removeItem(oauthReturnToKey);
        globalThis.history.replaceState({}, document.title, safeOAuthReturnTo(returnTo));
      } catch (caughtError) {
        if (cancelled) {
          return;
        }
        clearOAuthTransaction();
        setError(caughtError instanceof Error ? caughtError.message : "Sign-in failed.");
        setStatus("unauthenticated");
      }
    }

    void completeOAuthIfPresent();

    return () => {
      cancelled = true;
    };
  }, [session]);

  useEffect(() => {
    if (!session?.createdAt || !session.expiresIn) {
      return;
    }

    const expiresAtMs = (session.createdAt + session.expiresIn) * 1000;
    const timeout = globalThis.setTimeout(() => {
      if (Date.now() < expiresAtMs) {
        return;
      }
      clearAuthSession();
      setSession(null);
      setStatus("unauthenticated");
      setError("Your session expired. Sign in again.");
    }, sessionExpiryCheckDelay(expiresAtMs));

    return () => globalThis.clearTimeout(timeout);
  }, [session]);

  useEffect(() => {
    function syncSession(event: StorageEvent) {
      if (event.key !== authSessionKey) {
        return;
      }
      const nextSession = readAuthSession();
      setSession(nextSession);
      setStatus(nextSession ? "authenticated" : "unauthenticated");
      if (nextSession) {
        setError(null);
      }
    }

    globalThis.addEventListener("storage", syncSession);
    return () => globalThis.removeEventListener("storage", syncSession);
  }, []);

  const signIn = useCallback(async () => {
    const config = getOAuthConfig();
    if (!config) {
      setError("Sign-in is not configured yet.");
      return;
    }

    globalThis.sessionStorage.setItem(oauthReturnToKey, currentOAuthReturnTo());
    const url = await createAuthorizationUrl(config);
    globalThis.location.assign(url.toString());
  }, []);

  const signOut = useCallback(async () => {
    const sessionToRevoke = session;
    clearAuthSession();
    clearOAuthTransaction();
    setSession(null);
    setStatus("unauthenticated");
    setError(null);

    const config = getOAuthConfig();
    if (!sessionToRevoke?.accessToken || !config) {
      return;
    }

    try {
      await revokeAccessToken(config, sessionToRevoke.accessToken);
    } catch (caughtError) {
      console.warn("OAuth token revocation failed", caughtError);
    }
  }, [session]);

  const value = useMemo(
    () => ({ session, status, error, signIn, signOut }),
    [error, session, signIn, signOut, status],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

function currentOAuthReturnTo() {
  return `${globalThis.location.pathname}${globalThis.location.search}${globalThis.location.hash}` || "/";
}

function safeOAuthReturnTo(value: string | null) {
  if (!value || !value.startsWith("/")) {
    return "/";
  }
  if (value.startsWith("//")) {
    return "/";
  }
  return value;
}

function sessionExpiryCheckDelay(expiresAtMs: number) {
  const maxSafeBrowserDelay = 24 * 60 * 60 * 1000;
  return Math.min(Math.max(0, expiresAtMs - Date.now()), maxSafeBrowserDelay);
}

function getOAuthExchangePromise(code: string, exchange: () => Promise<AuthSession>) {
  const existingPromise = oauthExchangePromises.get(code);
  if (existingPromise) {
    return existingPromise;
  }

  const nextPromise = exchange();
  oauthExchangePromises.set(code, nextPromise);
  return nextPromise;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return context;
}
