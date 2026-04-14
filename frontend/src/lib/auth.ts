import { useState } from "react";
import { type AuthSession, type Role, readAuth } from "../storage";

export function useRequiredAuth(role: Role): AuthSession | null {
  const [auth] = useState(() => readAuth());
  if (!auth || auth.role !== role) return null;
  return auth;
}
