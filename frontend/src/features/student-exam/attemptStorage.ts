import type { Exam } from "../../api";
import type { AuthSession } from "../../storage";
import { readJSON } from "../../storage";

export type Attempt = {
  examId: string;
  account: string;
  startedAt: number;
  endAt: number;
  currentQuestion: number;
  answers: Record<string, number>;
  lastSavedAt: number | null;
  status: "in_progress" | "expired";
};

export function loadAttempt(auth: AuthSession, exam: Exam, attemptKey: string): Attempt {
  const saved = readJSON<Attempt>(localStorage, attemptKey);
  const now = Date.now();
  if (saved && saved.examId === exam.id) {
    return { ...saved, answers: saved.answers || {}, currentQuestion: Number.isInteger(saved.currentQuestion) ? saved.currentQuestion : 0 };
  }
  const initial: Attempt = {
    examId: exam.id,
    account: auth.account,
    startedAt: now,
    endAt: now + exam.durationSeconds * 1000,
    currentQuestion: 0,
    answers: {},
    lastSavedAt: null,
    status: "in_progress",
  };
  localStorage.setItem(attemptKey, JSON.stringify(initial));
  return initial;
}
