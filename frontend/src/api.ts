export type StudentDashboard = {
  profile: {
    displayName: string;
    className: string;
    email: string;
    status: string;
  };
  summary: {
    availableCount: number;
    plannedCount: number;
    latestScore: string;
  };
  availableExams: StudentExamSummary[];
  plannedExams: PlannedExam[];
  history: HistoryRecord[];
};

export type StudentExamSummary = {
  id: string;
  title: string;
  status: string;
  meta: string;
  duration: string;
};

export type PlannedExam = {
  title: string;
  time: string;
  detail: string;
};

export type HistoryRecord = {
  id: string;
  title: string;
  date: string;
  score: string;
  duration: string;
};

export type Exam = {
  id: string;
  title: string;
  durationSeconds: number;
  questions: ExamQuestion[];
};

export type ExamQuestion = {
  title: string;
  answers: string[];
};

export type Review = {
  title: string;
  score: string;
  duration: string;
  questions: ReviewQuestion[];
};

export type ReviewQuestion = {
  title: string;
  answers: string[];
  correctAnswer: number;
  selectedAnswer: number;
};

export type TeacherDashboard = {
  profile: {
    displayName: string;
    department: string;
  };
  exams: TeacherExamSummary[];
};

export type TeacherExamSummary = {
  id: string;
  title: string;
  status: string;
  targetClass: string;
  startTime: string;
  average: number;
  submitted: number;
  total: number;
};

export type TeacherExamDetail = TeacherExamSummary & {
  metrics: Metric[];
  tables: Record<string, StatisticsTable>;
  students?: StudentAttemptDetail[];
};

export type Metric = {
  label: string;
  value: string;
};

export type StatisticsTable = {
  title: string;
  columns: string[];
  rows: string[][];
};

export type StudentAttemptDetail = {
  name: string;
  progress: string;
  warning: string;
  score: string;
  duration: string;
  wrongItems: WrongItem[];
};

export type WrongItem = {
  question: string;
  selected: string;
  correct: string;
  note: string;
};

async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(path, { cache: "no-store" });
  if (!response.ok) {
    throw new Error(`Cannot load ${path}`);
  }
  return response.json() as Promise<T>;
}

export function getStudentDashboard(account: string) {
  return getJSON<StudentDashboard>(`/api/student/dashboard?account=${encodeURIComponent(account)}`);
}

export function getExam(examID: string) {
  return getJSON<Exam>(`/api/student/exams/${encodeURIComponent(examID)}`);
}

export function getReview(reviewID: string) {
  return getJSON<Review>(`/api/student/reviews/${encodeURIComponent(reviewID)}`);
}

export function getTeacherDashboard(account: string) {
  return getJSON<TeacherDashboard>(`/api/teacher/dashboard?account=${encodeURIComponent(account)}`);
}

export function getTeacherExam(examID: string) {
  return getJSON<TeacherExamDetail>(`/api/teacher/exams/${encodeURIComponent(examID)}`);
}
