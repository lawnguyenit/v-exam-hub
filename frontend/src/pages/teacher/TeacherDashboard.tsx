import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { FormEvent, ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { getTeacherDashboard, getTeacherExam, updateTeacherProfile } from "../../api";
import { TeacherDetail } from "../../features/teacher-statistics/TeacherDetail";
import { TeacherDetailModal } from "../../features/teacher-statistics/TeacherDetailModal";
import { useRequiredAuth } from "../../lib/auth";
import { examTypeClass, statusClass } from "../../lib/format";
import { Brand } from "../../shared/Brand";

export function TeacherDashboard() {
  const auth = useRequiredAuth("teacher");
  const queryClient = useQueryClient();
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("all");
  const [selectedExamID, setSelectedExamID] = useState("");
  const [statMode, setStatMode] = useState("top_students");
  const [modalContent, setModalContent] = useState<ReactNode | null>(null);
  const [isProfileOpen, setIsProfileOpen] = useState(false);
  const [profileForm, setProfileForm] = useState({ displayName: "", department: "", email: "", phone: "" });
  const [profileMessage, setProfileMessage] = useState("");

  if (!auth) return <Navigate to="/" replace />;
  const account = auth.account;

  const dashboard = useQuery({ queryKey: ["teacher-dashboard", account], queryFn: () => getTeacherDashboard(account) });
  const profileMutation = useMutation({
    mutationFn: updateTeacherProfile,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["teacher-dashboard", account] });
      setProfileMessage("Đã lưu hồ sơ giáo viên.");
      setIsProfileOpen(false);
    },
    onError: (error) => setProfileMessage(error instanceof Error ? error.message : "Không lưu được hồ sơ."),
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
  const displayName = profile?.displayName || auth.displayName || account || "Giáo viên";
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

  return (
    <>
      <header className="teacher-topbar">
        <Brand to="/teacher" />
        <nav className="teacher-nav" aria-label="Điều hướng giáo viên">
          <Link className="nav-tab active" to="/teacher">Dashboard</Link>
          <Link className="nav-tab" to="/teacher/create">Tạo bài kiểm tra</Link>
          <Link className="nav-tab" to="/teacher/students">Sinh viên</Link>
        </nav>
        <button className="account-chip account-chip-button" type="button" onClick={() => setIsProfileOpen(true)}>
          <span className="avatar">{initials}</span>
          <div>
            <strong>{displayName}</strong>
            <small>{profile?.department || "Chưa cập nhật khoa/bộ môn"}</small>
          </div>
        </button>
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
              <option value="all">Tất cả trạng thái</option>
              <option value="Đang mở">Đang mở</option>
              <option value="Lịch dự kiến">Lịch dự kiến</option>
              <option value="Đã đóng">Đã đóng</option>
              <option value="Bản nháp">Bản nháp</option>
            </select>
          </div>
          <div className="teacher-exam-list">
            {filtered.map((exam) => (
              <button className={`teacher-exam-card ${activeExamID === exam.id ? "active" : ""}`} type="button" key={exam.id} onClick={() => setSelectedExamID(exam.id)}>
                <div className="exam-badges">
                  <span className={`status-badge ${statusClass(exam.status)}`}>{exam.status}</span>
                  <span className={`type-badge ${examTypeClass(exam.examType)}`}>{exam.examType}</span>
                </div>
                <h3>{exam.title}</h3>
                <div className="exam-meta">
                  <span>{exam.targetClass}</span><span>{exam.startTime}</span><span>{exam.submitted}/{exam.total} đã nộp</span><span>TB {exam.average}</span>
                </div>
              </button>
            ))}
            {!filtered.length && <p className="empty-note">Không có bài thi phù hợp bộ lọc.</p>}
          </div>
        </aside>
      </main>
      {modalContent && <TeacherDetailModal content={modalContent} onClose={() => setModalContent(null)} />}
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
