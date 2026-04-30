import { useEffect, useState } from "react";
import { type AuthSession, type Role, readAuth } from "../storage";

export function useRequiredAuth(role: Role): AuthSession | null {
  const [auth, setAuth] = useState(() => readAuth());

  useEffect(() => {
    function syncAuth(event: StorageEvent) {
      if (event.key === "examhub:auth") {
        setAuth(readAuth());
      }
    }

    window.addEventListener("storage", syncAuth);
    return () => window.removeEventListener("storage", syncAuth);
  }, []);

  if (!auth || auth.role !== role) return null;
  return auth;
}
