import type {
  AccessCodeResult,
  ExamCreatePayload,
  ExamLiveSnapshot,
  QuestionBankDeleteResult,
  QuestionBankItem,
  StudentImportResult,
  TeacherClass,
  TeacherClassDetail,
  TeacherDashboard,
  TeacherExamDetail,
} from "./types";
import { getJSON, sendJSON } from "./client";

export function getTeacherDashboard(account: string) {
  return getJSON<TeacherDashboard>(`/api/teacher/dashboard?account=${encodeURIComponent(account)}`);
}

export function getTeacherExam(examID: string) {
  return getJSON<TeacherExamDetail>(`/api/teacher/exams/${encodeURIComponent(examID)}`);
}

export function updateTeacherProfile(payload: { username: string; displayName: string; department: string; email: string; phone: string }) {
  return sendJSON<TeacherDashboard["profile"]>("/api/teacher/profile", "POST", payload);
}

export function getTeacherClasses() {
  return getJSON<TeacherClass[]>("/api/teacher/classes");
}

export function getTeacherClassDetail(classID: number) {
  return getJSON<TeacherClassDetail>(`/api/teacher/classes/${encodeURIComponent(classID)}`);
}

export function updateTeacherClass(classID: number, payload: { classCode: string; className: string }) {
  return sendJSON<TeacherClass>(`/api/teacher/classes/${encodeURIComponent(classID)}`, "PATCH", payload);
}

export function deleteTeacherClass(classID: number) {
  return sendJSON<{ ok: boolean }>(`/api/teacher/classes/${encodeURIComponent(classID)}`, "DELETE");
}

export function removeTeacherClassMember(classID: number, userID: number) {
  return sendJSON<{ ok: boolean }>(`/api/teacher/classes/${encodeURIComponent(classID)}/members/${encodeURIComponent(userID)}`, "DELETE");
}

export function getTeacherQuestionBank(account?: string) {
  const query = account ? `?account=${encodeURIComponent(account)}` : "";
  return getJSON<QuestionBankItem[]>(`/api/teacher/question-bank${query}`);
}

export async function deleteTeacherQuestionBank(sourceID: number, account?: string) {
  const query = account ? `?account=${encodeURIComponent(account)}` : "";
  const response = await fetch(`/api/teacher/question-bank/${encodeURIComponent(sourceID)}${query}`, {
    method: "DELETE",
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<QuestionBankDeleteResult>;
}

export function getTeacherExamSnapshot(examID: string) {
  return getJSON<ExamLiveSnapshot>(`/api/teacher/exams/${encodeURIComponent(examID)}/snapshot`);
}

export function generateTeacherExamAccessCode(examID: string) {
  return sendJSON<AccessCodeResult>(`/api/teacher/exams/${encodeURIComponent(examID)}/access-code`, "POST");
}

export function createTeacherExam(payload: ExamCreatePayload) {
  return sendJSON<{ id: string; questionCount: number; status: string }>("/api/teacher/exams/create", "POST", payload);
}

export function deleteTeacherExam(examID: string) {
  return sendJSON<{ ok: boolean }>(`/api/teacher/exams/${encodeURIComponent(examID)}`, "DELETE");
}

export function importTeacherClassStudents(payload: { classCode: string; className: string; rows: string }) {
  return sendJSON<StudentImportResult>("/api/teacher/classes/import-students", "POST", payload);
}

export function updateTeacherStudentPassword(payload: { username: string; studentCode: string; password: string }) {
  return sendJSON<{ ok: boolean }>("/api/teacher/students/password", "POST", payload);
}

export async function importTeacherClassStudentsFile(payload: { classCode: string; className: string; file: File }) {
  const formData = new FormData();
  formData.append("classCode", payload.classCode);
  formData.append("className", payload.className);
  formData.append("file", payload.file);

  const response = await fetch("/api/teacher/classes/import-students", {
    method: "POST",
    body: formData,
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<StudentImportResult>;
}

