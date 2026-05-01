import { ApiError } from "./types";
import type { LoginResult } from "./types";
import { getJSON } from "./client";

export async function login(payload: { username: string; password: string; role: "student" | "teacher" | "admin" }) {
  const response = await fetch("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new ApiError(response.status, await response.text());
  }
  return response.json() as Promise<LoginResult>;
}

export function getCurrentSession() {
  return getJSON<LoginResult>("/api/auth/me");
}

export async function logout() {
  await fetch("/api/auth/logout", { method: "POST" });
}

