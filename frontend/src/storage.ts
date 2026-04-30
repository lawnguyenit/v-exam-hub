export type Role = "student" | "teacher" | "admin";

export type AuthSession = {
  account: string;
  role: Role;
  signedInAt: number;
  displayName?: string;
};

const authKey = "examhub:auth";

export function readJSON<T>(storage: Storage, key: string): T | null {
  try {
    const raw = storage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : null;
  } catch {
    return null;
  }
}

export function readAuth(): AuthSession | null {
  const auth = readJSON<AuthSession>(localStorage, authKey);
  if (auth) return auth;

  const legacyAuth = readJSON<AuthSession>(sessionStorage, authKey);
  if (legacyAuth) {
    writeAuth(legacyAuth);
    sessionStorage.removeItem(authKey);
  }
  return legacyAuth;
}

export function writeAuth(auth: AuthSession) {
  localStorage.setItem(authKey, JSON.stringify(auth));
}

export function clearAuth() {
  localStorage.removeItem(authKey);
  sessionStorage.removeItem(authKey);
}
