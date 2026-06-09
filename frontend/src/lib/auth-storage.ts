import type { AuthSession } from "@/types/auth";

export const authSessionKey = "gargoyle.auth.session";

export function readAuthSession(): AuthSession | null {
  const rawSession = readStoredSession();
  if (!rawSession) {
    return null;
  }

  try {
    const session = JSON.parse(rawSession) as Partial<AuthSession>;
    if (!session.accessToken || !session.tokenType || isExpired(session)) {
      clearAuthSession();
      return null;
    }
    return session as AuthSession;
  } catch {
    clearAuthSession();
    return null;
  }
}

export function writeAuthSession(session: AuthSession) {
  localStorage.setItem(authSessionKey, JSON.stringify(session));
  sessionStorage.removeItem(authSessionKey);
}

export function clearAuthSession() {
  localStorage.removeItem(authSessionKey);
  sessionStorage.removeItem(authSessionKey);
}

function readStoredSession() {
  const persistentSession = localStorage.getItem(authSessionKey);
  if (persistentSession) {
    return persistentSession;
  }

  const legacyTabSession = sessionStorage.getItem(authSessionKey);
  if (legacyTabSession) {
    localStorage.setItem(authSessionKey, legacyTabSession);
    sessionStorage.removeItem(authSessionKey);
  }
  return legacyTabSession;
}

function isExpired(session: Partial<AuthSession>) {
  if (!session.createdAt || !session.expiresIn) {
    return false;
  }

  const expiresAtMs = (session.createdAt + session.expiresIn) * 1000;
  return Date.now() >= expiresAtMs;
}
