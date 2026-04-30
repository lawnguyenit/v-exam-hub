import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { Link, Navigate, useSearchParams } from "react-router-dom";
import { type StudentDashboard as StudentDashboardData, getStudentDashboard, logout } from "../../api";
import { useRequiredAuth } from "../../lib/auth";
import { clearAuth } from "../../lib/authStorage";
import { Brand } from "../../shared/Brand";

export function StudentDashboard() {
  const auth = useRequiredAuth("student");
  const [searchParams, setSearchParams] = useSearchParams();
  const initialView = searchParams.get("view") || "overview";
  const [view, setView] = useState(["overview", "planned", "history", "profile"].includes(initialView) ? initialView : "overview");

  if (!auth) return <Navigate to="/" replace />;

  const dashboard = useQuery({
    queryKey: ["student-dashboard", auth.account],
    queryFn: () => getStudentDashboard(auth.account),
  });
  const initials = (auth.account || "SV").slice(0, 2).toUpperCase();
  const data = dashboard.data;

  function switchView(nextView: string) {
    setView(nextView);
    setSearchParams(nextView === "overview" ? {} : { view: nextView });
  }

  async function logoutStudent() {
    try {
      await logout();
    } catch {
      // Clear local state even if the server is unreachable.
    }
    clearAuth();
  }

  return (
    <>
      <header className="student-topbar">
        <Brand to="/student" />
        <nav className="student-nav" aria-label="Điều hướng sinh viên">
          {[
            ["overview", "Tổng quan"],
            ["planned", "Lịch dự kiến"],
            ["history", "Lịch sử"],
            ["profile", "Hồ sơ"],
          ].map(([key, label]) => (
            <button key={key} className={`nav-tab ${view === key ? "active" : ""}`} type="button" onClick={() => switchView(key)}>
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
          <Link className="ghost-btn" to="/" onClick={logoutStudent}>Đổi tư cách</Link>
        </section>

        {dashboard.isLoading && <article className="exam-card">Đang tải dashboard...</article>}
        {dashboard.isError && <article className="exam-card">Không thể tải dashboard. Hãy thử lại sau.</article>}
        {data && (
          <>
            <section className="summary-strip" aria-label="Tóm tắt nhanh">
              <article><span>Bài có thể làm</span><strong>{data.summary.availableCount}</strong></article>
              <article><span>Lịch dự kiến</span><strong>{data.summary.plannedCount}</strong></article>
              <article><span>Điểm gần nhất</span><strong>{data.summary.latestScore}</strong></article>
              <article><span>Lịch sử đã lưu</span><strong>{data.history.length}</strong></article>
            </section>
            <StudentOverview active={view === "overview"} data={data} account={auth.account} />
            <StudentPlanned active={view === "planned"} data={data} />
            <StudentHistory active={view === "history"} data={data} />
            <StudentProfile active={view === "profile"} data={data} account={auth.account} initials={initials} />
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
          {data.availableExams.map((exam) => (
            <article className="exam-card" key={exam.id}>
              <div>
                <p className="eyebrow">{exam.status}</p>
                <h3>{exam.title}</h3>
                <p>{exam.meta} Thời lượng {exam.duration}.</p>
              </div>
              <Link className="primary-btn" to={`/student/exam?id=${encodeURIComponent(exam.id)}`}>Vào làm bài</Link>
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

function StudentPlanned({ active, data }: { active: boolean; data: StudentDashboardData }) {
  return (
    <section className={`view-panel ${active ? "active" : ""}`}>
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
  );
}

function StudentHistory({ active, data }: { active: boolean; data: StudentDashboardData }) {
  const [historyQuery, setHistoryQuery] = useState("");
  const filteredHistory = data.history.filter((record) => (
    `${record.title} ${record.date} ${record.score}`.toLowerCase().includes(historyQuery.toLowerCase())
  ));

  return (
    <section className={`view-panel ${active ? "active" : ""}`}>
      <div className="section-head">
        <div><p className="eyebrow">Lịch sử bài thi</p><h2>Xem lại kết quả và chi tiết</h2></div>
        <input
          className="history-search"
          value={historyQuery}
          onChange={(event) => setHistoryQuery(event.target.value)}
          type="search"
          placeholder="Tìm theo tên bài kiểm tra"
        />
      </div>
      <div className="history-table" role="table" aria-label="Lịch sử bài thi">
        <div className="history-row header" role="row">
          <span>Bài thi</span><span>Ngày thi</span><span>Điểm</span><span>Thời gian</span><span></span>
        </div>
        {filteredHistory.map((record) => (
          <div className="history-row" role="row" key={record.id}>
            <span>{record.title}</span><span>{record.date}</span><span>{record.score}</span><span>{record.duration}</span>
            <Link className="ghost-btn" to={`/student/review?id=${encodeURIComponent(record.id)}`}>Xem chi tiết</Link>
          </div>
        ))}
        {!filteredHistory.length && <div className="history-row"><span>Không có lịch sử phù hợp.</span></div>}
      </div>
    </section>
  );
}

function StudentProfile({ active, data, account, initials }: { active: boolean; data: StudentDashboardData; account: string; initials: string }) {
  return (
    <section className={`view-panel ${active ? "active" : ""}`}>
      <div className="section-head"><div><p className="eyebrow">Hồ sơ cá nhân</p><h2>Thông tin tài khoản</h2></div></div>
      <div className="profile-layout">
        <article className="profile-photo">
          <span className="avatar large">{initials}</span>
          <button className="ghost-btn" type="button">Đổi ảnh đại diện</button>
        </article>
        <article className="profile-card">
          <div className="profile-line"><span>Họ và tên</span><strong>{data.profile.displayName}</strong></div>
          <div className="profile-line"><span>Tài khoản</span><strong>{account}</strong></div>
          <div className="profile-line"><span>Lớp</span><strong>{data.profile.className}</strong></div>
          <div className="profile-line"><span>Email trường</span><strong>{data.profile.email}</strong></div>
          <div className="profile-line"><span>Trạng thái</span><strong>{data.profile.status}</strong></div>
        </article>
      </div>
    </section>
  );
}


