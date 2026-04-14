import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { useMemo, useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { getTeacherDashboard, getTeacherExam } from "../../api";
import { TeacherDetail } from "../../features/teacher-statistics/TeacherDetail";
import { TeacherDetailModal } from "../../features/teacher-statistics/TeacherDetailModal";
import { useRequiredAuth } from "../../lib/auth";
import { examTypeClass, statusClass } from "../../lib/format";
import { Brand } from "../../shared/Brand";

export function TeacherDashboard() {
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
          </div>
        </aside>
      </main>
      {modalContent && <TeacherDetailModal content={modalContent} onClose={() => setModalContent(null)} />}
    </>
  );
}
