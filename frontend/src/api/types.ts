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

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

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
