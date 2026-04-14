import { useQuery } from "@tanstack/react-query";
import type { CSSProperties, FormEvent, ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";
import { Link, Navigate, Route, Routes, useNavigate, useSearchParams } from "react-router-dom";
import {
  type Exam,
  type ReviewQuestion,
  type StatisticsTable,
  type StudentDashboard as StudentDashboardData,
  type StudentAttemptDetail,
  type TeacherExamDetail,
  getExam,
  getReview,
  getStudentDashboard,
  getTeacherDashboard,
  getTeacherExam,
} from "./api";
import { type AuthSession, type Role, clearAuth, readAuth, readJSON, writeAuth } from "./storage";

type Attempt = {
  examId: string;
  account: string;
  startedAt: number;
  endAt: number;
  currentQuestion: number;
  answers: Record<string, number>;
  lastSavedAt: number | null;
  status: "in_progress" | "expired";
};

const statLabels: Record<string, string> = {
  top_students: "Sinh viên làm tốt nhất",
  score_distribution: "Phân bố điểm",
  question_difficulty: "Câu dễ sai nhất",
  live_status: "Trạng thái phòng thi",
};

function App() {
  return (
    <Routes>
      <Route path="/" element={<RoleSelect />} />
      <Route path="/login" element={<Navigate to="/" replace />} />
      <Route path="/login/student" element={<LoginPage role="student" />} />
      <Route path="/login/teacher" element={<LoginPage role="teacher" />} />
      <Route path="/student" element={<StudentDashboard />} />
      <Route path="/student/exam" element={<StudentExam />} />
      <Route path="/student/review" element={<StudentReview />} />
      <Route path="/teacher" element={<TeacherDashboard />} />
      <Route path="/teacher/create" element={<TeacherCreateExam />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

function Brand({ to = "/" }: { to?: string }) {
  return (
    <Link className="brand" to={to} aria-label="ExamHub">
      <span className="brand-mark">E</span>
      <span>ExamHub</span>
    </Link>
  );
}

function RoleSelect() {
  return (
    <main className="login-page login-shell" aria-label="Chọn tư cách đăng nhập">
      <section className="login-box role-box">
        <Brand />
        <div className="login-copy">
          <p className="eyebrow">Tài khoản do trường cấp</p>
          <h1>Chọn tư cách đăng nhập</h1>
          <p>Không mở đăng ký tự do để tránh lạm dụng điểm và nhầm quyền truy cập.</p>
        </div>
        <nav className="login-bars" aria-label="Vai trò đăng nhập">
          <Link className="login-bar teacher" to="/login/teacher">
            <span>Đăng nhập với tư cách giáo viên</span>
            <strong>Quản lý lớp thi</strong>
          </Link>
          <Link className="login-bar student" to="/login/student">
            <span>Đăng nhập với tư cách sinh viên / học sinh</span>
            <strong>Vào dashboard làm bài</strong>
          </Link>
        </nav>
      </section>
    </main>
  );
}

function LoginPage({ role }: { role: Role }) {
  const navigate = useNavigate();
  const [account, setAccount] = useState("");
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState(
    role === "teacher"
      ? "Luồng giáo viên hiện là placeholder, nhưng vẫn đi qua bước đăng nhập để giữ đúng flow."
      : "Bản UI test chấp nhận tài khoản/mật khẩu bất kỳ. Backend thật sẽ xác thực bằng dữ liệu trường cấp.",
  );

  useEffect(() => {
    sessionStorage.setItem("examhub:lastRole", role);
  }, [role]);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!account.trim() || !password.trim()) {
      setMessage("Cần nhập đủ tài khoản và mật khẩu được cấp.");
      return;
    }
    writeAuth({ account: account.trim(), role, signedInAt: Date.now() });
    navigate(role === "teacher" ? "/teacher" : "/student");
  }

  return (
    <main className="login-page login-shell" aria-label={`Đăng nhập ${role === "teacher" ? "giáo viên" : "sinh viên"}`}>
      <section className="login-box form-box">
        <Brand />
        <form className="credential-form standalone" onSubmit={submit}>
          <div className="form-head">
            <p className="eyebrow">{role === "teacher" ? "Giáo viên" : "Sinh viên / học sinh"}</p>
            <h1>{role === "teacher" ? "Đăng nhập quản lý lớp thi" : "Đăng nhập để làm bài"}</h1>
            <p>
              {role === "teacher"
                ? "Tài khoản giáo viên do nhà trường cấp, không mở đăng ký tự do."
                : "Tiến trình cũ sẽ được mở lại nếu đăng nhập cùng tài khoản trên trình duyệt này."}
            </p>
          </div>
          <label htmlFor="accountInput">Tài khoản</label>
          <input
            id="accountInput"
            value={account}
            onChange={(event) => setAccount(event.target.value)}
            autoComplete="username"
            placeholder={role === "teacher" ? "VD: gv-cntt-01" : "VD: lnit hoặc lawnguyenit"}
            required
          />
          <label htmlFor="passwordInput">Mật khẩu</label>
          <input
            id="passwordInput"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete="current-password"
            placeholder={role === "teacher" ? "Mật khẩu giáo viên" : "Mật khẩu hoặc mã truy cập"}
            required
          />
          <button className="primary-btn" type="submit">Vào dashboard</button>
          <p className="form-message">{message}</p>
        </form>
        <Link className="back-link" to="/">Chọn tư cách khác</Link>
      </section>
    </main>
  );
}

function useRequiredAuth(role: Role): AuthSession | null {
  const [auth] = useState(() => readAuth());
  if (!auth || auth.role !== role) return null;
  return auth;
}

function StudentDashboard() {
  const auth = useRequiredAuth("student");
  const [view, setView] = useState("overview");

  if (!auth) return <Navigate to="/" replace />;

  const dashboard = useQuery({
    queryKey: ["student-dashboard", auth.account],
    queryFn: () => getStudentDashboard(auth.account),
  });
  const initials = (auth.account || "SV").slice(0, 2).toUpperCase();
  const attempt = readJSON<Attempt>(localStorage, `examhub:attempt:${auth.account}:go-basics-demo`);
  const answered = attempt?.answers ? Object.keys(attempt.answers).length : 0;
  const data = dashboard.data;

  return (
    <>
      <header className="student-topbar">
        <Brand />
        <nav className="student-nav" aria-label="Điều hướng sinh viên">
          {[
            ["overview", "Tổng quan"],
            ["planned", "Lịch dự kiến"],
            ["history", "Lịch sử"],
            ["profile", "Hồ sơ"],
          ].map(([key, label]) => (
            <button key={key} className={`nav-tab ${view === key ? "active" : ""}`} type="button" onClick={() => setView(key)}>
              {label}
            </button>
          ))}
        </nav>
        <div className="account-chip">
          <span className="avatar">{initials}</span>
          <div>
            <strong>{data?.profile.displayName || "Sinh viên"}</strong>
            <small>Tài khoản: {auth.account}</small>
          </div>
        </div>
      </header>

      <main className="student-dashboard">
        <section className="dashboard-hero" aria-label="Tổng quan sinh viên">
          <div>
            <p className="eyebrow">Dashboard sinh viên / học sinh</p>
            <h1>Theo dõi bài kiểm tra của bạn</h1>
            <p className="lead">Vào bài đang mở, xem lịch dự kiến, kiểm tra lịch sử điểm và chỉnh thông tin cơ bản từ tài khoản được trường cấp.</p>
          </div>
          <Link className="ghost-btn" to="/" onClick={clearAuth}>Đổi tư cách</Link>
        </section>

        {dashboard.isLoading && <article className="exam-card">Đang tải dashboard...</article>}
        {dashboard.isError && <article className="exam-card">Không thể tải dashboard. Hãy thử lại sau.</article>}
        {data && (
          <>
            <section className="summary-strip" aria-label="Tóm tắt nhanh">
              <article><span>Bài có thể làm</span><strong>{data.summary.availableCount}</strong></article>
              <article><span>Lịch dự kiến</span><strong>{data.summary.plannedCount}</strong></article>
              <article><span>Điểm gần nhất</span><strong>{data.summary.latestScore}</strong></article>
              <article><span>Tiến trình đang lưu</span><strong>{answered}/12</strong></article>
            </section>
            <StudentOverview active={view === "overview"} data={data} account={auth.account} />
            <section className={`view-panel ${view === "planned" ? "active" : ""}`}>
              <div className="section-head"><div><p className="eyebrow">Lịch dự kiến</p><h2>Bài đã được giáo viên lên lịch</h2></div></div>
              <div className="schedule-list">
                {data.plannedExams.map((exam) => (
                  <article key={`${exam.title}-${exam.time}`}>
                    <span>{exam.time}</span>
                    <strong>{exam.title}</strong>
                    <small>{exam.detail}</small>
                  </article>
                ))}
              </div>
            </section>
            <section className={`view-panel ${view === "history" ? "active" : ""}`}>
              <div className="section-head"><div><p className="eyebrow">Lịch sử bài thi</p><h2>Xem lại kết quả và chi tiết</h2></div></div>
              <div className="history-table" role="table" aria-label="Lịch sử bài thi">
                <div className="history-row header" role="row">
                  <span>Bài thi</span><span>Ngày thi</span><span>Điểm</span><span>Thời gian</span><span></span>
                </div>
                {data.history.map((record) => (
                  <div className="history-row" role="row" key={record.id}>
                    <span>{record.title}</span><span>{record.date}</span><span>{record.score}</span><span>{record.duration}</span>
                    <Link className="ghost-btn" to={`/student/review?id=${encodeURIComponent(record.id)}`}>Xem chi tiết</Link>
                  </div>
                ))}
              </div>
            </section>
            <section className={`view-panel ${view === "profile" ? "active" : ""}`}>
              <div className="section-head"><div><p className="eyebrow">Hồ sơ cá nhân</p><h2>Thông tin tài khoản</h2></div></div>
              <div className="profile-layout">
                <article className="profile-photo">
                  <span className="avatar large">{initials}</span>
                  <button className="ghost-btn" type="button">Đổi ảnh đại diện</button>
                </article>
                <article className="profile-card">
                  <div className="profile-line"><span>Họ và tên</span><strong>{data.profile.displayName}</strong></div>
                  <div className="profile-line"><span>Tài khoản</span><strong>{auth.account}</strong></div>
                  <div className="profile-line"><span>Lớp</span><strong>{data.profile.className}</strong></div>
                  <div className="profile-line"><span>Email trường</span><strong>{data.profile.email}</strong></div>
                  <div className="profile-line"><span>Trạng thái</span><strong>{data.profile.status}</strong></div>
                </article>
              </div>
            </section>
          </>
        )}
      </main>
    </>
  );
}

function StudentOverview({ active, data, account }: { active: boolean; data: StudentDashboardData; account: string }) {
  return (
    <section className={`view-panel ${active ? "active" : ""}`}>
      <div className="section-head"><div><p className="eyebrow">Tổng quan</p><h2>Việc cần làm hôm nay</h2></div></div>
      <div className="dashboard-grid">
        <div className="exam-list" aria-label="Bài kiểm tra có thể làm">
          {data.availableExams.map((exam, index) => (
            <article className={`exam-card ${index === 0 ? "featured" : ""}`} key={exam.id}>
              <div>
                <p className="eyebrow">{exam.status}</p>
                <h3>{exam.title}</h3>
                <p>{exam.meta} Thời lượng {exam.duration}.</p>
              </div>
              <Link className={index === 0 ? "primary-btn" : "ghost-btn"} to={`/student/exam?id=${encodeURIComponent(exam.id)}`}>
                Vào làm bài
              </Link>
            </article>
          ))}
        </div>
        <article className="profile-card">
          <p className="eyebrow">Hồ sơ nhanh</p>
          <div className="profile-line"><span>Họ và tên</span><strong>{data.profile.displayName}</strong></div>
          <div className="profile-line"><span>Lớp</span><strong>{data.profile.className}</strong></div>
          <div className="profile-line"><span>Mã sinh viên</span><strong>{account.toUpperCase()}</strong></div>
        </article>
      </div>
    </section>
  );
}

function StudentExam() {
  const auth = useRequiredAuth("student");
  const [params] = useSearchParams();
  const examID = params.get("id") || "go-basics-demo";

  if (!auth) return <Navigate to="/" replace />;

  const examQuery = useQuery({ queryKey: ["exam", examID], queryFn: () => getExam(examID) });
  return examQuery.data ? <ExamWorkspace auth={auth} exam={examQuery.data} /> : (
    <PageShell backTo="/student">
      <main className="exam-page">
        <article className="exam-panel">{examQuery.isError ? "Không thể tải bài kiểm tra" : "Đang tải bài kiểm tra..."}</article>
      </main>
    </PageShell>
  );
}

function ExamWorkspace({ auth, exam }: { auth: AuthSession; exam: Exam }) {
  const attemptKey = `examhub:attempt:${auth.account}:${exam.id}`;
  const [attempt, setAttempt] = useState(() => loadAttempt(auth, exam, attemptKey));
  const [now, setNow] = useState(Date.now());
  const currentQuestion = exam.questions[attempt.currentQuestion];
  const expired = now >= attempt.endAt;
  const secondsLeft = Math.max(0, Math.floor((attempt.endAt - now) / 1000));
  const selectedAnswer = attempt.answers[String(attempt.currentQuestion)];

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    localStorage.setItem(attemptKey, JSON.stringify(attempt));
  }, [attempt, attemptKey]);

  useEffect(() => {
    if (expired && attempt.status !== "expired") {
      setAttempt((current) => ({ ...current, status: "expired" }));
    }
  }, [attempt.status, expired]);

  function saveAnswer(answerIndex: number) {
    setAttempt((current) => ({
      ...current,
      answers: { ...current.answers, [current.currentQuestion]: answerIndex },
      lastSavedAt: Date.now(),
    }));
  }

  function moveQuestion(offset: number) {
    setAttempt((current) => ({
      ...current,
      currentQuestion: Math.min(exam.questions.length - 1, Math.max(0, current.currentQuestion + offset)),
    }));
  }

  return (
    <PageShell backTo="/student">
      <main className="exam-page">
        <section className="exam-workspace" aria-label="Bài đang làm">
          <article className="exam-panel">
            <div className="panel-head">
              <div>
                <p className="eyebrow">Bài đang làm</p>
                <h1>{exam.title}</h1>
              </div>
              <div className="timer" aria-live="polite">
                <span>Còn lại</span>
                <strong>{formatSeconds(secondsLeft)}</strong>
              </div>
            </div>
            <div className="question-box" aria-live="polite">
              <p className="question-meta">Câu {attempt.currentQuestion + 1} / {exam.questions.length}</p>
              <h2>{currentQuestion.title}</h2>
              <div className="answers">
                {currentQuestion.answers.map((answer, answerIndex) => (
                  <label key={answer}>
                    <input
                      type="radio"
                      name="answer"
                      value={answerIndex}
                      checked={selectedAnswer === answerIndex}
                      disabled={expired}
                      onChange={() => saveAnswer(answerIndex)}
                    />
                    {answer}
                  </label>
                ))}
              </div>
            </div>
            <div className="question-nav" aria-label="Danh sách câu hỏi">
              {exam.questions.map((question, index) => (
                <button
                  className={`question-pill ${index === attempt.currentQuestion ? "active" : ""} ${Object.hasOwn(attempt.answers, String(index)) ? "done" : ""}`}
                  type="button"
                  key={question.title}
                  onClick={() => setAttempt((current) => ({ ...current, currentQuestion: index }))}
                >
                  {index + 1}
                </button>
              ))}
            </div>
            <div className="exam-actions">
              <button className="ghost-btn" type="button" disabled={expired} onClick={() => moveQuestion(-1)}>Câu trước</button>
              <button className="primary-btn" type="button" disabled={expired} onClick={() => moveQuestion(1)}>Lưu và tiếp tục</button>
            </div>
          </article>
          <aside className="student-side">
            <article className="status-panel">
              <p className="eyebrow">Trạng thái</p>
              <h2>{expired ? "Đã hết thời gian" : attempt.lastSavedAt ? "Đã khôi phục tiến trình" : "Bắt đầu tiến trình mới"}</h2>
              <p>{expired ? "Tiến trình vẫn được giữ lại để đối chiếu, nhưng không nên cho sửa đáp án sau khi hết giờ." : "Tiến trình sẽ được tự động lưu khi chọn đáp án hoặc chuyển câu."}</p>
            </article>
            <article className="status-panel">
              <p className="eyebrow">Tiến trình hiện tại</p>
              <div className="metric-strip">
                <div><span>Đã trả lời</span><strong>{Object.keys(attempt.answers).length}/{exam.questions.length}</strong></div>
                <div><span>Lưu gần nhất</span><strong>{attempt.lastSavedAt ? formatTime(attempt.lastSavedAt) : "--:--"}</strong></div>
              </div>
            </article>
          </aside>
        </section>
      </main>
    </PageShell>
  );
}

function StudentReview() {
  const auth = useRequiredAuth("student");
  const [params] = useSearchParams();
  const reviewID = params.get("id") || "go-intro";
  const reviewQuery = useQuery({ queryKey: ["review", reviewID], queryFn: () => getReview(reviewID) });

  if (!auth) return <Navigate to="/" replace />;

  return (
    <PageShell backTo="/student">
      <main className="review-page">
        <section className="review-hero">
          <div>
            <p className="eyebrow">Xem lại bài thi</p>
            <h1>{reviewQuery.data?.title || (reviewQuery.isError ? "Không thể tải bài xem lại" : "Đang tải bài thi")}</h1>
          </div>
          <div className="review-score">
            <span>Điểm</span>
            <strong>{reviewQuery.data?.score || "--"}</strong>
            <small>{reviewQuery.data ? `Thời gian làm bài: ${reviewQuery.data.duration}` : "--"}</small>
          </div>
        </section>
        <section className="review-list" aria-label="Danh sách câu đã làm">
          {reviewQuery.data?.questions.map((question, index) => <ReviewCard question={question} index={index} key={question.title} />)}
        </section>
      </main>
    </PageShell>
  );
}

function ReviewCard({ question, index }: { question: ReviewQuestion; index: number }) {
  return (
    <article className="review-question">
      <p className="eyebrow">Câu {index + 1}</p>
      <h2>{question.title}</h2>
      <div className="review-answers">
        {question.answers.map((answer, answerIndex) => {
          const correct = answerIndex === question.correctAnswer;
          const selected = answerIndex === question.selectedAnswer;
          return (
            <div className={`review-answer ${correct ? "correct" : selected ? "wrong" : ""}`} key={answer}>
              <span>{String.fromCharCode(65 + answerIndex)}. {answer}</span>
              <strong>{correct ? "Đáp án đúng" : selected ? "Bạn đã chọn" : ""}</strong>
            </div>
          );
        })}
      </div>
    </article>
  );
}

function TeacherDashboard() {
  const auth = useRequiredAuth("teacher");
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("all");
  const [selectedExamID, setSelectedExamID] = useState("");
  const [statMode, setStatMode] = useState("top_students");
  const [modalContent, setModalContent] = useState<ReactNode | null>(null);

  if (!auth) return <Navigate to="/" replace />;

  const dashboard = useQuery({ queryKey: ["teacher-dashboard", auth.account], queryFn: () => getTeacherDashboard(auth.account) });
  const exams = dashboard.data?.exams || [];
  const activeExamID = selectedExamID || exams[0]?.id || "";
  const detail = useQuery({ queryKey: ["teacher-exam", activeExamID], queryFn: () => getTeacherExam(activeExamID), enabled: Boolean(activeExamID) });
  const filtered = useMemo(() => {
    const result = exams.filter((exam) => {
      const matchQuery = !query || `${exam.title} ${exam.targetClass}`.toLowerCase().includes(query.toLowerCase());
      const matchStatus = status === "all" || exam.status === status;
      return matchQuery && matchStatus;
    });
    return status === "all" && !query ? result.slice(0, 3) : result;
  }, [exams, query, status]);
  const initials = (auth.account || "GV").slice(0, 2).toUpperCase();

  return (
    <>
      <header className="teacher-topbar">
        <Brand />
        <nav className="teacher-nav" aria-label="Điều hướng giáo viên">
          <Link className="nav-tab active" to="/teacher">Dashboard</Link>
          <Link className="nav-tab" to="/teacher/create">Tạo bài kiểm tra</Link>
        </nav>
        <div className="account-chip">
          <span className="avatar">{initials}</span>
          <div>
            <strong>{dashboard.data?.profile.displayName || "Giáo viên"}</strong>
            <small>{dashboard.data?.profile.department || "Khoa CNTT"}</small>
          </div>
        </div>
      </header>
      <main className="teacher-dashboard">
        <section className="teacher-workspace">
          <div className="dashboard-hero">
            <div>
              <p className="eyebrow">Dashboard giáo viên</p>
              <h1>Quản lý bài thi và thống kê lớp</h1>
              <p className="lead">Theo dõi bài đã tạo, xem bảng thống kê theo tiêu chí, chuẩn bị bài mới từ file Word/PDF qua bước AI format.</p>
            </div>
            <Link className="primary-btn" to="/teacher/create">Tạo bài kiểm tra</Link>
          </div>
          <TeacherDetail exam={detail.data} statMode={statMode} onStatMode={setStatMode} onOpen={setModalContent} />
        </section>
        <aside className="exam-rail" aria-label="Danh sách bài thi đã tạo">
          <div className="rail-head">
            <div>
              <p className="eyebrow">Bài đã tạo</p>
              <h2>Danh sách bài thi</h2>
            </div>
          </div>
          <div className="rail-tools">
            <input value={query} onChange={(event) => setQuery(event.target.value)} type="search" placeholder="Tìm theo tên bài hoặc lớp" />
            <select value={status} onChange={(event) => setStatus(event.target.value)} aria-label="Lọc trạng thái bài thi">
              <option value="all">Tất cả - 3 gần nhất</option>
              <option value="Đang mở">Đang mở</option>
              <option value="Lịch dự kiến">Lịch dự kiến</option>
              <option value="Thi thử">Thi thử</option>
            </select>
          </div>
          <div className="teacher-exam-list">
            {filtered.map((exam) => (
              <button className={`teacher-exam-card ${activeExamID === exam.id ? "active" : ""}`} type="button" key={exam.id} onClick={() => setSelectedExamID(exam.id)}>
                <span className={`status-badge ${statusClass(exam.status)}`}>{exam.status}</span>
                <h3>{exam.title}</h3>
                <div className="exam-meta">
                  <span>{exam.targetClass}</span><span>{exam.startTime}</span><span>{exam.submitted}/{exam.total} đã nộp</span><span>TB {exam.average}</span>
                </div>
              </button>
            ))}
          </div>
        </aside>
      </main>
      {modalContent && (
        <div className="modal-backdrop open" onClick={() => setModalContent(null)}>
          <section className="student-detail-modal" role="dialog" aria-modal="true" onClick={(event) => event.stopPropagation()}>
            <button className="modal-close" type="button" aria-label="Đóng" onClick={() => setModalContent(null)}>×</button>
            {modalContent}
          </section>
        </div>
      )}
    </>
  );
}

function TeacherDetail({
  exam,
  statMode,
  onStatMode,
  onOpen,
}: {
  exam?: TeacherExamDetail;
  statMode: string;
  onStatMode: (mode: string) => void;
  onOpen: (content: ReactNode) => void;
}) {
  const active = activeStatisticsTable(exam, statMode);
  const rows = active ? buildTeacherRows(exam, active.key, active.table) : [];

  return (
    <section className="exam-detail-panel">
      <div className="section-head">
        <div>
          <p className="eyebrow">Chi tiết bài thi</p>
          <h2>{exam?.title || "Chọn một bài thi ở danh sách bên phải"}</h2>
          <p>{exam ? `${exam.status} - ${exam.targetClass} - ${exam.startTime}` : "Bảng thống kê sẽ hiển thị theo option bạn chọn."}</p>
        </div>
        <label className="stat-selector">
          <span>Hiển thị bảng</span>
          <select value={statMode} onChange={(event) => onStatMode(event.target.value)}>
            {Object.entries(statLabels).map(([key, label]) => <option value={key} key={key}>{label}</option>)}
          </select>
        </label>
      </div>
      <div className="metric-grid">
        {exam?.metrics.map((metric) => (
          <article className="metric-card" key={metric.label}>
            <span>{metric.label}</span>
            <strong>{metric.value}</strong>
          </article>
        ))}
      </div>
      {active && (
        <div className="stats-scroll">
          <div className="stats-table">
            <div className="stats-row header" style={{ "--columns": active.table.columns.length + 1 } as CSSProperties}>
              {active.table.columns.map((column) => <span key={column}>{column}</span>)}
              <span>Chi tiết</span>
            </div>
            {rows.map((row) => (
              <div className="stats-row" style={{ "--columns": active.table.columns.length + 1 } as CSSProperties} key={`${row.rowIndex}-${row.cells.join("-")}`}>
                {row.cells.map((cell, index) => <span key={`${cell}-${index}`}>{cell}</span>)}
                <button className="table-action" type="button" onClick={() => onOpen(renderTeacherModal(exam, active.key, active.table, row.rowIndex))}>Xem</button>
              </div>
            ))}
          </div>
        </div>
      )}
    </section>
  );
}

function TeacherCreateExam() {
  const auth = useRequiredAuth("teacher");
  const [fileState, setFileState] = useState("Chưa có file");
  const [result, setResult] = useState("Sau khi backend AI sẵn sàng, vùng này sẽ hiển thị bản preview câu hỏi đã chuẩn hóa.");

  if (!auth) return <Navigate to="/" replace />;

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (fileState === "Chưa có file") {
      setResult("Cần chọn file đề thi trước khi gửi qua AI format.");
      return;
    }
    setResult("Đã nhận file ở UI mock. Backend sau này sẽ gửi file sang pipeline AI để trích câu hỏi, đáp án, điểm và lời giải thích.");
  }

  return (
    <PageShell backTo="/teacher">
      <main className="create-page">
        <section className="create-hero">
          <div>
            <p className="eyebrow">Tạo bài kiểm tra</p>
            <h1>Upload đề và chuẩn hóa trước khi mở cho lớp</h1>
            <p className="lead">Luồng dự kiến: giáo viên tải file Word/PDF, AI format về chuẩn hệ thống, giáo viên duyệt lại câu hỏi rồi mới xuất bản.</p>
          </div>
        </section>
        <section className="create-layout">
          <form className="create-form" onSubmit={submit}>
            <label htmlFor="examTitle">Tên bài kiểm tra</label>
            <input id="examTitle" placeholder="VD: Cơ sở dữ liệu - Kiểm tra 15 phút" required />
            <label htmlFor="examMode">Loại bài</label>
            <select id="examMode"><option>Thi thử</option><option>Thi chính thức</option></select>
            <label htmlFor="targetClass">Lớp áp dụng</label>
            <select id="targetClass"><option>CNTT K48</option><option>CNTT K49</option><option>Mạng máy tính K47</option></select>
            <label htmlFor="startTime">Thời gian mở bài</label>
            <input id="startTime" type="datetime-local" />
            <label htmlFor="duration">Thời lượng</label>
            <input id="duration" type="number" min="5" defaultValue="45" />
            <label htmlFor="examFile">File đề thi</label>
            <input
              id="examFile"
              type="file"
              accept=".doc,.docx,.pdf,.txt,.xlsx"
              onChange={(event) => {
                const file = event.target.files?.[0];
                setFileState(file ? `${file.name} - ${(file.size / 1024).toFixed(1)} KB` : "Chưa có file");
              }}
            />
            <p className="form-note">Hỗ trợ trước: Word, PDF, TXT, XLSX. File sẽ đi qua bước AI format trước khi tạo ngân hàng câu hỏi.</p>
            <button className="primary-btn" type="submit">Gửi lên để AI format</button>
          </form>
          <aside className="ai-pipeline">
            <p className="eyebrow">AI format pipeline</p>
            <h2>Chuẩn hóa về cấu trúc hệ thống</h2>
            {[
              ["1. Nhận file", fileState],
              ["2. Trích câu hỏi", "Nhận dạng câu, đáp án, điểm, lời giải thích."],
              ["3. Giáo viên duyệt", "Chỉnh lại câu sai format trước khi lưu."],
              ["4. Xuất bản", "Gán lớp, lịch mở bài, thời lượng và trạng thái."],
            ].map(([title, text], index) => (
              <div className={`pipeline-step ${index === 0 ? "active" : ""}`} key={title}>
                <strong>{title}</strong>
                <span>{text}</span>
              </div>
            ))}
            <div className="pipeline-result">{result}</div>
          </aside>
        </section>
      </main>
    </PageShell>
  );
}

function PageShell({ backTo, children }: { backTo: string; children: ReactNode }) {
  return (
    <>
      <header className="app-header">
        <Brand to={backTo} />
        <Link className="ghost-btn" to={backTo}>Về dashboard</Link>
      </header>
      {children}
    </>
  );
}

function loadAttempt(auth: AuthSession, exam: Exam, attemptKey: string): Attempt {
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

function formatSeconds(seconds: number) {
  return `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}

function formatTime(timestamp: number) {
  return new Date(timestamp).toLocaleTimeString("vi-VN", { hour: "2-digit", minute: "2-digit" });
}

function statusClass(status: string) {
  if (status === "Đang mở") return "status-open";
  if (status === "Lịch dự kiến") return "status-scheduled";
  if (status === "Thi thử") return "status-practice";
  return "status-default";
}

function activeStatisticsTable(exam: TeacherExamDetail | undefined, statMode: string) {
  if (!exam) return null;
  const key = exam.tables[statMode] ? statMode : Object.keys(exam.tables)[0];
  return { key, table: exam.tables[key] };
}

function buildTeacherRows(exam: TeacherExamDetail | undefined, key: string, table: StatisticsTable) {
  if (key === "live_status") {
    return (exam?.students || []).map((student, rowIndex) => ({ cells: [student.name, student.progress, student.warning], rowIndex }));
  }
  return table.rows.map((cells, rowIndex) => ({ cells, rowIndex }));
}

function renderTeacherModal(exam: TeacherExamDetail | undefined, key: string, table: StatisticsTable, rowIndex: number) {
  if (!exam) return <p>Chưa có dữ liệu.</p>;
  if (key === "live_status") return renderStudentAttempt(exam.students?.[rowIndex]);
  if (key === "top_students") {
    const name = table.rows[rowIndex]?.[0];
    return renderStudentAttempt(exam.students?.find((student) => student.name === name));
  }
  if (key === "score_distribution") {
    const row = table.rows[rowIndex];
    const students = studentsForRow(exam.students || [], rowIndex, table.rows.length);
    return (
      <>
        <p className="eyebrow">Chi tiết nhóm điểm</p>
        <h2>Khoảng {row[0]}</h2>
        <div className="student-detail-meta"><span>Số sinh viên: {row[1]}</span><span>Tỷ lệ: {row[2]}</span></div>
        <StudentPreviewList students={students} />
      </>
    );
  }
  if (key === "question_difficulty") {
    const row = table.rows[rowIndex];
    const students = studentsForRow(exam.students || [], rowIndex, table.rows.length);
    return (
      <>
        <p className="eyebrow">Chi tiết câu hỏi</p>
        <h2>{row[0]} - {row[2]}</h2>
        <div className="student-detail-meta"><span>Tỷ lệ sai: {row[1]}</span><span>Sinh viên liên quan: {students.length}</span></div>
        <StudentPreviewList students={students} />
      </>
    );
  }
  return (
    <>
      <p className="eyebrow">Chi tiết thống kê</p>
      <h2>{table.rows[rowIndex]?.[0] || "Chưa có dữ liệu"}</h2>
      <div className="wrong-list">
        {(table.rows[rowIndex] || []).map((cell, index) => (
          <div className="wrong-item" key={`${cell}-${index}`}>
            <strong>{table.columns[index] || "Mục"}</strong>
            <span>{cell}</span>
          </div>
        ))}
      </div>
    </>
  );
}

function renderStudentAttempt(student?: StudentAttemptDetail) {
  if (!student) return <p>Chưa có dữ liệu sinh viên.</p>;
  return (
    <>
      <p className="eyebrow">Chi tiết sinh viên</p>
      <h2>{student.name}</h2>
      <div className="student-detail-meta">
        <span>Tiến trình: {student.progress}</span><span>Điểm: {student.score}</span><span>Thời gian: {student.duration}</span><span>Cảnh báo: {student.warning}</span>
      </div>
      <div className="wrong-list">
        {student.wrongItems.map((item) => (
          <div className="wrong-item" key={item.question}>
            <strong>{item.question}</strong>
            <span>Đã chọn: {item.selected}</span>
            <span>Đáp án đúng: {item.correct}</span>
            <p>{item.note}</p>
          </div>
        ))}
      </div>
    </>
  );
}

function StudentPreviewList({ students }: { students: StudentAttemptDetail[] }) {
  if (!students.length) return <p className="empty-note">Chưa có sinh viên phù hợp hàng thống kê này.</p>;
  return (
    <div className="wrong-list">
      {students.map((student) => (
        <div className="wrong-item" key={student.name}>
          <strong>{student.name}</strong>
          <span>Điểm: {student.score}</span>
          <span>Thời gian: {student.duration}</span>
          <span>Cảnh báo: {student.warning}</span>
          <p>{student.wrongItems[0]?.note || "Chưa có ghi chú sai chi tiết."}</p>
        </div>
      ))}
    </div>
  );
}

function studentsForRow(students: StudentAttemptDetail[], rowIndex: number, rowCount: number) {
  return students.filter((_, index) => index % rowCount === rowIndex);
}

export default App;
