import type { AttemptState, Exam, Review, StudentDashboard } from "./types";
import { getJSON, sendJSON } from "./client";

export function getStudentDashboard(account: string) {
  return getJSON<StudentDashboard>(`/api/student/dashboard?account=${encodeURIComponent(account)}`);
}

export function getExam(examID: string) {
  return getJSON<Exam>(`/api/student/exams/${encodeURIComponent(examID)}`);
}

export function getReview(reviewID: string) {
  return getJSON<Review>(`/api/student/reviews/${encodeURIComponent(reviewID)}`);
}

export function startStudentAttempt(payload: { account: string; examId: string; accessCode?: string }) {
  return sendJSON<AttemptState>("/api/student/attempts/start", "POST", payload);
}

export function saveStudentAnswer(payload: { attemptId: number; questionIndex: number; answerIndex: number }) {
  return sendJSON<AttemptState>("/api/student/attempts/save", "POST", payload);
}

export function syncStudentAttemptDraft(payload: { attemptId: number; currentQuestion: number; answers: Record<string, number> }) {
  return sendJSON<AttemptState>("/api/student/attempts/sync", "POST", payload);
}

export function updateStudentAttemptProgress(payload: { attemptId: number; questionIndex: number }) {
  return sendJSON<AttemptState>("/api/student/attempts/progress", "POST", payload);
}

export function submitStudentAttempt(payload: { attemptId: number }) {
  return sendJSON<AttemptState>("/api/student/attempts/submit", "POST", payload);
}

