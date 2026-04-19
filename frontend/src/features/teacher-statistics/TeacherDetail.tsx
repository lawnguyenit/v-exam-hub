import type { CSSProperties, ReactNode } from "react";
import type { TeacherExamDetail } from "../../api";
import { activeStatisticsTable, buildTeacherRows, renderTeacherModal, statLabels, teacherTableColumns } from "./teacherStatistics";

export function TeacherDetail({
  exam,
  statMode,
  onStatMode,
  onOpen,
  onEdit,
  onAccessCode,
  onSnapshot,
}: {
  exam?: TeacherExamDetail;
  statMode: string;
  onStatMode: (mode: string) => void;
  onOpen: (content: ReactNode) => void;
  onEdit: (examID: string) => void;
  onAccessCode: (examID: string) => void;
  onSnapshot: (examID: string) => void;
}) {
  const active = activeStatisticsTable(exam, statMode);
  const rows = active ? buildTeacherRows(exam, active.key, active.table) : [];
  const columns = active ? teacherTableColumns(active.key, active.table) : [];
  const metrics = exam?.metrics.filter((metric) => metric.label !== "Thời gian TB" && metric.label !== "Thời gian trung bình");
  const shouldShowEmptyState = !exam || exam.status === "Lịch dự kiến" || !active || rows.length === 0;

  return (
    <section className="exam-detail-panel">
      <div className="section-head">
        <div>
          <p className="eyebrow">Chi tiết bài thi</p>
          <h2>{exam?.title || "Chọn một bài thi ở danh sách bên phải"}</h2>
          <p>{exam ? `${exam.status} - ${exam.targetClass} - ${exam.startTime}` : "Bảng thống kê sẽ hiển thị theo option bạn chọn."}</p>
        </div>
        <div className="detail-actions">
          <label className="stat-selector">
            <span>Hiển thị bảng</span>
            <select value={statMode} onChange={(event) => onStatMode(event.target.value)}>
              {Object.entries(statLabels).map(([key, label]) => <option value={key} key={key}>{label}</option>)}
            </select>
          </label>
          {exam && (
            <div className="detail-action-row">
              {(exam.examMode === "official" || exam.examMode === "attendance") && (
                <button className="table-action" type="button" onClick={() => onAccessCode(exam.id)}>Tạo mã</button>
              )}
              <button className="table-action" type="button" onClick={() => onSnapshot(exam.id)}>Snapshot</button>
              <a className="table-action" href={`/api/teacher/exams/${encodeURIComponent(exam.id)}/export`}>Export XLSX</a>
            </div>
          )}
        </div>
      </div>
      <div className="metric-grid">
        {exam
          ? metrics?.map((metric) => (
              <article className="metric-card" key={metric.label}>
                <span>{metric.label}</span>
                <strong>{metric.value}</strong>
              </article>
            ))
          : defaultMetrics().map((metric) => (
              <article className="metric-card muted" key={metric.label}>
                <span>{metric.label}</span>
                <strong>{metric.value}</strong>
              </article>
            ))}
      </div>
      <div className="stats-scroll">
        {shouldShowEmptyState ? (
          <TeacherStatsEmptyState exam={exam} mode={active?.key || statMode} onEdit={onEdit} />
        ) : (
          <div className="stats-table">
            <div className="stats-row header" style={{ "--columns": columns.length + 1 } as CSSProperties}>
              {columns.map((column) => <span key={column}>{column}</span>)}
              <span>Chi tiết</span>
            </div>
            {rows.map((row) => (
              <div className="stats-row" style={{ "--columns": columns.length + 1 } as CSSProperties} key={`${row.rowIndex}-${row.cells.join("-")}`}>
                {row.cells.map((cell, index) => <span key={`${cell}-${index}`}>{cell}</span>)}
                <button className="table-action" type="button" onClick={() => onOpen(renderTeacherModal(exam, active.key, active.table, row.rowIndex))}>Xem</button>
              </div>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}

function TeacherStatsEmptyState({ exam, mode, onEdit }: { exam?: TeacherExamDetail; mode: string; onEdit: (examID: string) => void }) {
  const copy = emptyStateCopy(exam, mode);

  return (
    <article className="stats-empty-state">
      <p className="eyebrow">{copy.eyebrow}</p>
      <h3>{copy.title}</h3>
      <p>{copy.message}</p>
      <div className="empty-actions">
        <button className="ghost-btn" type="button" disabled={!exam} onClick={() => exam && onEdit(exam.id)}>Xem cấu hình bài thi</button>
        <button className="primary-btn" type="button" disabled>{copy.action}</button>
      </div>
    </article>
  );
}

function emptyStateCopy(exam: TeacherExamDetail | undefined, mode: string) {
  if (!exam) {
    return {
      eyebrow: "Chưa chọn bài thi",
      title: "Chọn một bài thi ở danh sách bên phải",
      message: "Khu thống kê sẽ giữ nguyên kích thước và chỉ thay đổi dữ liệu bên trong khi bạn chọn bài.",
      action: "Chưa có dữ liệu",
    };
  }

  if (exam.status === "Lịch dự kiến") {
    return {
      eyebrow: "Chưa mở bài",
      title: "Bài này chưa đến giờ làm",
      message: "Thống kê sinh viên sẽ xuất hiện sau khi ca thi mở và có sinh viên bắt đầu làm bài.",
      action: "Đang chờ mở",
    };
  }

  if (mode === "live_status") {
    return {
      eyebrow: "Phòng thi trống",
      title: "Chưa có sinh viên bắt đầu làm bài",
      message: "Vùng này sẽ tự chuyển thành danh sách cuộn khi có sinh viên tham gia.",
      action: "Đang theo dõi",
    };
  }

  return {
    eyebrow: "Chưa có thống kê",
    title: "Chưa đủ dữ liệu để dựng bảng",
    message: "Bảng thống kê sẽ được lấp đầy khi hệ thống nhận bài làm hoặc kết quả chấm.",
    action: "Chờ dữ liệu",
  };
}

function defaultMetrics() {
  return [
    { label: "Đã nộp", value: "--" },
    { label: "Điểm cao nhất", value: "--" },
    { label: "Điểm trung bình", value: "--" },
    { label: "Số lượt làm", value: "--" },
  ];
}
