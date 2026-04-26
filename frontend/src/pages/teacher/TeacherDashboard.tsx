import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { FormEvent, ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";
import { Link, Navigate, useNavigate } from "react-router-dom";
import { deleteTeacherExam, generateTeacherExamAccessCode, getTeacherDashboard, getTeacherExam, getTeacherExamSnapshot, logout, updateTeacherProfile } from "../../api";
import { TeacherDetail } from "../../features/teacher-statistics/TeacherDetail";
import { TeacherDetailModal } from "../../features/teacher-statistics/TeacherDetailModal";
import { useRequiredAuth } from "../../lib/auth";
import { examTypeClass, statusClass } from "../../lib/format";
import { clearAuth } from "../../storage";
import { Brand } from "../../shared/Brand";

export function TeacherDashboard() {
  const auth = useRequiredAuth("teacher");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("all");
  const [selectedExamID, setSelectedExamID] = useState("");
  const [statMode, setStatMode] = useState("top_students");
  const [modalContent, setModalContent] = useState<ReactNode | null>(null);
  const [pendingDeleteExam, setPendingDeleteExam] = useState<{ id: string; title: string } | null>(null);
  const [isProfileOpen, setIsProfileOpen] = useState(false);
  const [profileForm, setProfileForm] = useState({ displayName: "", department: "", email: "", phone: "" });
  const [profileMessage, setProfileMessage] = useState("");

  const account = auth?.account || "";

  const dashboard = useQuery({ queryKey: ["teacher-dashboard", account], queryFn: () => getTeacherDashboard(account), enabled: Boolean(auth) });
  const profileMutation = useMutation({
    mutationFn: updateTeacherProfile,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["teacher-dashboard", account] });
      setProfileMessage("Đã lưu hồ sơ giáo viên.");
      setIsProfileOpen(false);
    },
    onError: (error) => setProfileMessage(error instanceof Error ? error.message : "Không lưu được hồ sơ."),
  });
  const deleteMutation = useMutation({
    mutationFn: deleteTeacherExam,
    onSuccess: async (_, examID) => {
      if (selectedExamID === examID) {
        setSelectedExamID("");
      }
      setPendingDeleteExam(null);
      await queryClient.invalidateQueries({ queryKey: ["teacher-dashboard", account] });
      await queryClient.invalidateQueries({ queryKey: ["teacher-exam"] });
    },
  });
  const exams = dashboard.data?.exams || [];
  const activeExamID = selectedExamID || exams[0]?.id || "";
  const detail = useQuery({ queryKey: ["teacher-exam", activeExamID], queryFn: () => getTeacherExam(activeExamID), enabled: Boolean(activeExamID) });
  const filtered = useMemo(() => {
    return exams.filter((exam) => {
      const matchQuery = !query || `${exam.title} ${exam.targetClass}`.toLowerCase().includes(query.toLowerCase());
      const matchStatus = status === "all" || exam.status === status;
      return matchQuery && matchStatus;
    });
  }, [exams, query, status]);
  const profile = dashboard.data?.profile;
  const displayName = profile?.displayName || auth?.displayName || account || "Giáo viên";
  const initials = initialsFrom(displayName);

  useEffect(() => {
    if (!profile) return;
    setProfileForm({
      displayName: profile.displayName || "",
      department: profile.department || "",
      email: profile.email || "",
      phone: profile.phone || "",
    });
  }, [profile]);

  function saveProfile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setProfileMessage("Đang lưu hồ sơ...");
    profileMutation.mutate({ username: account, ...profileForm });
  }

  async function openAccessCode(examID: string) {
    setModalContent(<p>Đang tạo mã truy cập...</p>);
    try {
      const result = await generateTeacherExamAccessCode(examID);
      setModalContent(<AccessCodeModalContent code={result.code} durationMinute={result.durationMinute} expiresAt={result.expiresAt} />);
    } catch (error) {
      setModalContent(<p>{error instanceof Error ? error.message : "Không tạo được mã truy cập."}</p>);
    }
  }

  async function logoutTeacher() {
    try {
      await logout();
    } catch {
      // Frontend session still needs to be cleared even if the network is unavailable.
    }
    clearAuth();
    navigate("/", { replace: true });
  }

  async function openSnapshot(examID: string) {
    setModalContent(<p>Đang tải snapshot phòng thi...</p>);
    try {
      const snapshot = await getTeacherExamSnapshot(examID);
      setModalContent(
        <section className="snapshot-modal-content">
          <p className="eyebrow">Snapshot phòng thi</p>
          <h2>Cập nhật {snapshot.generatedAt}</h2>
          <div className="snapshot-metrics">
            <span><strong>{snapshot.total}</strong>Tổng</span>
            <span><strong>{snapshot.inProgress}</strong>Đang làm</span>
            <span><strong>{snapshot.submitted}</strong>Đã làm</span>
            <span><strong>{snapshot.notStarted}</strong>Chưa làm</span>
          </div>
          <div className="snapshot-list">
            {snapshot.rows.map((row) => (
              <article key={`${row.studentCode}-${row.name}`}>
                <strong>{row.name}</strong>
                <span>{row.studentCode} - {row.status} - {row.attemptCount} lần - điểm {row.bestScore}</span>
                <small>Cập nhật: {row.lastSeen}</small>
              </article>
            ))}
          </div>
        </section>,
      );
    } catch (error) {
      setModalContent(<p>{error instanceof Error ? error.message : "Không tải được snapshot phòng thi."}</p>);
    }
  }

  if (!auth) return <Navigate to="/" replace />;

  return (
    <>
      <header className="teacher-topbar">
        <Brand to="/teacher" />
        <nav className="teacher-nav" aria-label="Điều hướng giáo viên">
          <Link className="nav-tab active" to="/teacher">Dashboard</Link>
          <Link className="nav-tab" to="/teacher/create">Tạo bài kiểm tra</Link>
          <Link className="nav-tab" to="/teacher/question-bank">Đề cương</Link>
          <Link className="nav-tab" to="/teacher/classes">Lớp</Link>
          <Link className="nav-tab" to="/teacher/students">Sinh viên</Link>
        </nav>
        <div className="teacher-account-actions">
          <button className="account-chip account-chip-button" type="button" onClick={() => setIsProfileOpen(true)}>
            <span className="avatar">{initials}</span>
            <div>
              <strong>{displayName}</strong>
              <small>{profile?.department || "Chưa cập nhật khoa/bộ môn"}</small>
            </div>
          </button>
          <button className="logout-btn" type="button" onClick={logoutTeacher}>Đăng xuất</button>
        </div>
      </header>
      <main className="teacher-dashboard">
        <section className="teacher-workspace">
          <div className="dashboard-hero">
            <div>
              <p className="eyebrow">Dashboard giáo viên</p>
              <h1>Quản lý bài thi và thống kê lớp</h1>
            </div>
            <Link className="primary-btn" to="/teacher/create">Tạo bài kiểm tra</Link>
          </div>
          <TeacherDetail
            exam={detail.data}
            statMode={statMode}
            onStatMode={setStatMode}
            onOpen={setModalContent}
            onEdit={(examID) => navigate(`/teacher/create?exam=${encodeURIComponent(examID)}`)}
            onAccessCode={openAccessCode}
            onSnapshot={openSnapshot}
          />
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
              <option value="all">Tất cả trạng thái</option>
              <option value="Đang mở">Đang mở</option>
              <option value="Lịch dự kiến">Lịch dự kiến</option>
              <option value="Đã đóng">Đã đóng</option>
              <option value="Bản nháp">Bản nháp</option>
            </select>
          </div>
          <div className="teacher-exam-list">
            {filtered.map((exam) => (
              <article className={`teacher-exam-card ${activeExamID === exam.id ? "active" : ""}`} key={exam.id}>
                <button className="teacher-exam-main" type="button" onClick={() => setSelectedExamID(exam.id)}>
                  <div className="exam-badges">
                    <span className={`status-badge ${statusClass(exam.status)}`}>{exam.status}</span>
                    <span className={`type-badge ${examTypeClass(exam.examType)}`}>{exam.examType}</span>
                  </div>
                  <h3>{exam.title}</h3>
                  <div className="exam-meta">
                    <span>{exam.targetClass}</span><span>{exam.startTime}</span><span>{exam.submitted}/{exam.total} đã nộp</span><span>TB {exam.average}</span>
                  </div>
                </button>
                <div className="exam-card-actions">
                  <button className="table-action" type="button" onClick={() => navigate(`/teacher/create?exam=${encodeURIComponent(exam.id)}`)}>Cấu hình</button>
                  <button
                    className="table-action danger"
                    type="button"
                    disabled={deleteMutation.isPending}
                    onClick={() => setPendingDeleteExam({ id: exam.id, title: exam.title })}
                  >
                    Xoá
                  </button>
                </div>
              </article>
            ))}
            {!filtered.length && <p className="empty-note">Không có bài thi phù hợp bộ lọc.</p>}
          </div>
        </aside>
      </main>
      {modalContent && <TeacherDetailModal content={modalContent} onClose={() => setModalContent(null)} />}
      {pendingDeleteExam && (
        <div className="modal-backdrop open" role="dialog" aria-modal="true" aria-label="Xác nhận xoá bài kiểm tra">
          <section className="teacher-confirm-modal">
            <p className="eyebrow">Xác nhận xoá</p>
            <h2>Xoá bài kiểm tra này?</h2>
            <p>Bài "{pendingDeleteExam.title}" sẽ bị xoá khỏi danh sách quản lý. Thao tác này chỉ nên dùng khi tạo nhầm bài.</p>
            <div className="modal-actions">
              <button className="ghost-btn" type="button" onClick={() => setPendingDeleteExam(null)}>Huỷ</button>
              <button
                className="primary-btn danger"
                type="button"
                disabled={deleteMutation.isPending}
                onClick={() => deleteMutation.mutate(pendingDeleteExam.id)}
              >
                {deleteMutation.isPending ? "Đang xoá..." : "Xoá bài"}
              </button>
            </div>
          </section>
        </div>
      )}
      {isProfileOpen && (
        <div className="modal-backdrop open" role="dialog" aria-modal="true" aria-label="Chỉnh hồ sơ giáo viên">
          <form className="teacher-profile-modal" onSubmit={saveProfile}>
            <button className="modal-close" type="button" onClick={() => setIsProfileOpen(false)}>×</button>
            <p className="eyebrow">Hồ sơ giáo viên</p>
            <h2>Thông tin tài khoản</h2>
            <div className="profile-account-row">
              <span className="avatar">{initials}</span>
              <div>
                <strong>{account}</strong>
                <small>{profile?.teacherCode || "Chưa có mã giáo viên"}</small>
              </div>
            </div>
            <label>
              Họ và tên
              <input value={profileForm.displayName} onChange={(event) => setProfileForm((current) => ({ ...current, displayName: event.target.value }))} required />
            </label>
            <label>
              Khoa / bộ môn
              <input value={profileForm.department} onChange={(event) => setProfileForm((current) => ({ ...current, department: event.target.value }))} />
            </label>
            <label>
              Email
              <input type="email" value={profileForm.email} onChange={(event) => setProfileForm((current) => ({ ...current, email: event.target.value }))} />
            </label>
            <label>
              Số điện thoại
              <input value={profileForm.phone} onChange={(event) => setProfileForm((current) => ({ ...current, phone: event.target.value }))} />
            </label>
            <div className="modal-actions">
              <button className="logout-btn" type="button" onClick={logoutTeacher}>Đăng xuất</button>
              <button className="ghost-btn" type="button" onClick={() => setIsProfileOpen(false)}>Đóng</button>
              <button className="primary-btn" type="submit" disabled={profileMutation.isPending}>{profileMutation.isPending ? "Đang lưu..." : "Lưu thông tin"}</button>
            </div>
            {profileMessage && <p className="student-import-message">{profileMessage}</p>}
          </form>
        </div>
      )}
    </>
  );
}

function initialsFrom(value: string) {
  const words = value.trim().split(/\s+/).filter(Boolean);
  if (words.length >= 2) return `${words[0][0]}${words[words.length - 1][0]}`.toUpperCase();
  return (value || "GV").slice(0, 2).toUpperCase();
}

function AccessCodeModalContent({ code, durationMinute, expiresAt }: { code: string; durationMinute: number; expiresAt: string }) {
  const [copied, setCopied] = useState(false);

  async function copyCode() {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      setCopied(false);
    }
  }

  return (
    <section className="snapshot-modal-content">
      <p className="eyebrow">Mã truy cập</p>
      <button className="access-code-copy" type="button" onClick={copyCode} title="Bấm để sao chép mã">
        <strong>{code}</strong>
        <span>{copied ? "Đã sao chép" : "Bấm để sao chép"}</span>
      </button>
      <p>Mã tồn tại {durationMinute} phút, hết hạn lúc {expiresAt}.</p>
      <p>Chỉ sinh viên thuộc lớp target và có mã này mới vào được bài chính thức hoặc điểm danh.</p>
    </section>
  );
}
