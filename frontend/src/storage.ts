export type Role = "student" | "teacher" | "admin";

export type AuthSession = {
  account: string;
  role: Role;
  signedInAt: number;
  displayName?: string;
};

export function readJSON<T>(storage: Storage, key: string): T | null {
  try {
    const raw = storage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : null;
  } catch {
    return null;
  }
}

export function readAuth(): AuthSession | null {
  return readJSON<AuthSession>(sessionStorage, "examhub:auth");
}

export function writeAuth(auth: AuthSession) {
  sessionStorage.setItem("examhub:auth", JSON.stringify(auth));
}

export function clearAuth() {
  sessionStorage.removeItem("examhub:auth");
}
