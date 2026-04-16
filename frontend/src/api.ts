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
    teacherCode: string;
    department: string;
    email: string;
    phone: string;
  };
  exams: TeacherExamSummary[];
};

export type TeacherExamSummary = {
  id: string;
  title: string;
  status: string;
  examType: string;
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
  studentCode: string;
  progress: string;
  warning: string;
  score: string;
  duration: string;
  attemptCount: number;
  attempts: AttemptDetail[];
  wrongItems: WrongItem[];
};

export type AttemptDetail = {
  attemptNo: number;
  score: string;
  duration: string;
  status: string;
  submittedAt: string;
  wrongItems?: WrongItem[];
};

export type WrongItem = {
  question: string;
  selected: string;
  correct: string;
  note: string;
};

export type LoginResult = {
  username: string;
  role: "student" | "teacher";
  displayName: string;
};

export type AttemptState = {
  attemptId: number;
  examId: string;
  startedAt: number;
  endAt: number;
  currentQuestion: number;
  answers: Record<string, number>;
  status: "in_progress" | "submitted" | "expired" | "cancelled";
  score?: string;
  lastSavedAt: number;
};

export type ImportFileInfo = {
  name: string;
  size: number;
  kind: string;
};

export type ImportExtractInfo = {
  status: string;
  text: string;
  needsOcr: boolean;
  needsConversion: boolean;
  warning: string;
  pageEstimate: number;
  imageCount: number;
  documentTitle: string;
  headingCandidates: string[];
};

export type ImportParsedOption = {
  label: string;
  content: string;
};

export type ImportParsedQuestion = {
  importItemId?: number;
  sourceOrder: number;
  content: string;
  options: ImportParsedOption[];
  correctLabel?: string;
  confidence: number;
  status: "pass" | "review" | "fail";
  warnings: string[];
};

export type ImportParseSummary = {
  total: number;
  passed: number;
  review: number;
  failed: number;
  averageConfidence: number;
};

export type ImportParseResult = {
  importBatchId: number;
  file: ImportFileInfo;
  extract: ImportExtractInfo;
  questions: ImportParsedQuestion[];
  summary: ImportParseSummary;
  message: string;
};

export type StudentImportResult = {
  classCode: string;
  className: string;
  created: number;
  updated: number;
  addedToClass: number;
  skipped: number;
  importedStudents: Array<{
    username: string;
    studentCode: string;
    fullName: string;
    temporaryPassword: string;
  }>;
  generatedPasswords: Array<{
    username: string;
    studentCode: string;
    fullName: string;
    password: string;
  }>;
  errors: string[];
};

export type TeacherClass = {
  id: number;
  classCode: string;
  className: string;
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

export async function updateTeacherProfile(payload: { username: string; displayName: string; department: string; email: string; phone: string }) {
  const response = await fetch("/api/teacher/profile", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<TeacherDashboard["profile"]>;
}

export async function login(payload: { username: string; password: string; role: "student" | "teacher" }) {
  const response = await fetch("/api/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<LoginResult>;
}

export async function startStudentAttempt(payload: { account: string; examId: string }) {
  const response = await fetch("/api/student/attempts/start", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<AttemptState>;
}

export async function saveStudentAnswer(payload: { attemptId: number; questionIndex: number; answerIndex: number }) {
  const response = await fetch("/api/student/attempts/save", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<AttemptState>;
}

export async function syncStudentAttemptDraft(payload: { attemptId: number; currentQuestion: number; answers: Record<string, number> }) {
  const response = await fetch("/api/student/attempts/sync", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<AttemptState>;
}

export async function updateStudentAttemptProgress(payload: { attemptId: number; questionIndex: number }) {
  const response = await fetch("/api/student/attempts/progress", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<AttemptState>;
}

export async function submitStudentAttempt(payload: { attemptId: number }) {
  const response = await fetch("/api/student/attempts/submit", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<AttemptState>;
}

export async function parseTeacherImport(file: File) {
  const formData = new FormData();
  formData.append("file", file);

  const response = await fetch("/api/teacher/import/parse", {
    method: "POST",
    body: formData,
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<ImportParseResult>;
}

export async function saveTeacherImportItem(importBatchId: number, question: ImportParsedQuestion) {
  const response = await fetch("/api/teacher/import/items/save", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ importBatchId, question }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<{ ok: boolean }>;
}

export async function approveTeacherImportPassItems(importBatchId: number) {
  const response = await fetch("/api/teacher/import/approve-pass", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ importBatchId }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<{
    importBatchId: number;
    approved: number;
    alreadyApproved: number;
    skipped: number;
    rejected: number;
  }>;
}

export function getTeacherClasses() {
  return getJSON<TeacherClass[]>("/api/teacher/classes");
}

export async function importTeacherClassStudents(payload: { classCode: string; className: string; rows: string }) {
  const response = await fetch("/api/teacher/classes/import-students", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<StudentImportResult>;
}

export async function updateTeacherStudentPassword(payload: { username: string; studentCode: string; password: string }) {
  const response = await fetch("/api/teacher/students/password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<{ ok: boolean }>;
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
