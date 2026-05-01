import type { TeacherCreateResult } from "./types";

export async function createAdminTeacher(payload: {
  adminUsername: string;
  username: string;
  password: string;
  teacherCode: string;
  fullName: string;
  email: string;
  phone: string;
  department: string;
}) {
  const response = await fetch("/api/admin/teachers", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<TeacherCreateResult>;
}

