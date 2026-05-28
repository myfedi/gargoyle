import type { AuthSession } from "@/types/auth";

const authSessionKey = "gargoyle.auth.session";

export function readAuthSession(): AuthSession | null {
  const rawSession = sessionStorage.getItem(authSessionKey);
  if (!rawSession) {
    return null;
  }

  try {
    const session = JSON.parse(rawSession) as Partial<AuthSession>;
    if (!session.accessToken || !session.tokenType) {
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
  sessionStorage.setItem(authSessionKey, JSON.stringify(session));
}

export function clearAuthSession() {
  sessionStorage.removeItem(authSessionKey);
}
