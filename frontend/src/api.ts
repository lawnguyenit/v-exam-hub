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
  examMode: "practice" | "official" | "attendance";
  requiresAccessCode: boolean;
  questions: ExamQuestion[];
};

export type ExamQuestion = {
  title: string;
  answers: string[];
  assetBatchId?: number;
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
  assetBatchId?: number;
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
  description: string;
  statusCode: string;
  examMode: "practice" | "official" | "attendance";
  classId: number;
  startValue: string;
  durationMinutes: number;
  maxAttemptsPerStudent: number;
  shuffleQuestions: boolean;
  shuffleOptions: boolean;
  showResultImmediately: boolean;
  allowReview: boolean;
  questionSourceId: number;
  questionCount: number;
  canEdit: boolean;
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
  role: "student" | "teacher" | "admin";
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
  duplicateCandidates?: ImportDuplicateCandidate[];
  message: string;
};

export type ImportDuplicateCandidate = {
  batchId: number;
  title: string;
  sourceName: string;
  existingQuestionCount: number;
  matchingQuestionCount: number;
  newQuestionCount: number;
  createdAt: string;
};

export type StudentImportResult = {
  classCode: string;
  className: string;
  created: number;
  updated: number;
  addedToClass: number;
  skipped: number;
  importedStudents: Array<{
    sourceRow?: number;
    username: string;
    studentCode: string;
    fullName: string;
    temporaryPassword: string;
  }>;
  generatedPasswords: Array<{
    sourceRow?: number;
    username: string;
    studentCode: string;
    fullName: string;
    password: string;
  }>;
  errors: string[];
  rowErrors?: Array<{
    sourceRow: number;
    studentCode: string;
    fullName: string;
    email: string;
    phone: string;
    username: string;
    password: string;
    message: string;
  }>;
};

export type TeacherCreateResult = {
  username: string;
  teacherCode: string;
  fullName: string;
  email: string;
  department: string;
  temporaryPassword: string;
  created: boolean;
};

export type TeacherClass = {
  id: number;
  classCode: string;
  className: string;
  memberCount?: number;
  examCount?: number;
};

export type TeacherClassDetail = TeacherClass & {
  memberCount: number;
  examCount: number;
  averageScore: string;
  members: Array<{
    userId: number;
    username: string;
    studentCode: string;
    fullName: string;
    email: string;
    phone: string;
    attemptCount: number;
    bestScore: string;
    lastSeen: string;
  }>;
  exams: Array<{
    id: number;
    title: string;
    status: string;
    submitted: number;
    total: number;
    average: string;
  }>;
};

export type QuestionBankItem = {
  id: number;
  title: string;
  sourceName: string;
  questionCount: number;
  createdAt: string;
};

export type QuestionBankDeleteResult = {
  id: number;
  archivedQuestions: number;
  deletedQuestions: number;
  removedBatch: boolean;
};

export type ExamCreatePayload = {
  examId?: string;
  createdBy: string;
  title: string;
  description?: string;
  examMode: "practice" | "official" | "attendance";
  classId: number;
  startTime: string;
  durationMinutes: number;
  maxAttemptsPerStudent: number;
  shuffleQuestions: boolean;
  shuffleOptions: boolean;
  showResultImmediately: boolean;
  allowReview: boolean;
  questionIds: number[];
  questionSourceId: number;
  questionCount: number;
};

export type AccessCodeResult = {
  examId: string;
  code: string;
  expiresAt: string;
  expiresAtUnix: number;
  durationMinute: number;
};

export type ExamLiveSnapshot = {
  examId: string;
  generatedAt: string;
  total: number;
  inProgress: number;
  submitted: number;
  notStarted: number;
  rows: Array<{
    studentCode: string;
    name: string;
    status: string;
    attemptCount: number;
    bestScore: string;
    lastSeen: string;
  }>;
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

export async function login(payload: { username: string; password: string; role: "student" | "teacher" | "admin" }) {
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

export async function logout() {
  await fetch("/api/auth/logout", {
    method: "POST",
  });
}

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

export async function startStudentAttempt(payload: { account: string; examId: string; accessCode?: string }) {
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

export async function parseTeacherImport(file: File, account?: string) {
  const formData = new FormData();
  formData.append("file", file);
  if (account) {
    formData.append("account", account);
  }

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

export async function createTeacherImportItem(importBatchId: number, question: ImportParsedQuestion) {
  const response = await fetch("/api/teacher/import/items/create", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ importBatchId, question }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<ImportParsedQuestion>;
}

export async function deleteTeacherImportItem(importBatchId: number, importItemId: number) {
  const response = await fetch("/api/teacher/import/items/delete", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ importBatchId, importItemId }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<{ ok: boolean }>;
}

export async function approveTeacherImportPassItems(importBatchId: number, targetBatchId?: number) {
  const response = await fetch("/api/teacher/import/approve-pass", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ importBatchId, targetBatchId }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<{
    importBatchId: number;
    targetBatchId?: number;
    approved: number;
    alreadyApproved: number;
    skipped: number;
    rejected: number;
    questionCount: number;
    questionIds: number[];
  }>;
}

export function getTeacherClasses() {
  return getJSON<TeacherClass[]>("/api/teacher/classes");
}

export function getTeacherClassDetail(classID: number) {
  return getJSON<TeacherClassDetail>(`/api/teacher/classes/${encodeURIComponent(classID)}`);
}

export async function updateTeacherClass(classID: number, payload: { classCode: string; className: string }) {
  const response = await fetch(`/api/teacher/classes/${encodeURIComponent(classID)}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<TeacherClass>;
}

export async function deleteTeacherClass(classID: number) {
  const response = await fetch(`/api/teacher/classes/${encodeURIComponent(classID)}`, { method: "DELETE" });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<{ ok: boolean }>;
}

export async function removeTeacherClassMember(classID: number, userID: number) {
  const response = await fetch(`/api/teacher/classes/${encodeURIComponent(classID)}/members/${encodeURIComponent(userID)}`, { method: "DELETE" });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<{ ok: boolean }>;
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

export async function generateTeacherExamAccessCode(examID: string) {
  const response = await fetch(`/api/teacher/exams/${encodeURIComponent(examID)}/access-code`, {
    method: "POST",
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<AccessCodeResult>;
}

export async function createTeacherExam(payload: ExamCreatePayload) {
  const response = await fetch("/api/teacher/exams/create", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<{ id: string; questionCount: number; status: string }>;
}

export async function deleteTeacherExam(examID: string) {
  const response = await fetch(`/api/teacher/exams/${encodeURIComponent(examID)}`, {
    method: "DELETE",
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return response.json() as Promise<{ ok: boolean }>;
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
