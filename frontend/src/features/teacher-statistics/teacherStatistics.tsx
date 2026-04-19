import { useMemo, useState, type ReactNode } from "react";
import type { StatisticsTable, StudentAttemptDetail, TeacherExamDetail } from "../../api";

export const statLabels: Record<string, string> = {
  top_students: "Sinh viên làm tốt nhất",
  score_distribution: "Phân bố điểm",
  question_difficulty: "Câu dễ sai nhất",
  live_status: "Trạng thái phòng thi",
};

export function activeStatisticsTable(exam: TeacherExamDetail | undefined, statMode: string) {
  if (!exam) return null;
  const key = exam.tables[statMode] ? statMode : Object.keys(exam.tables)[0];
  return { key, table: exam.tables[key] };
}

export function teacherTableColumns(key: string, table: StatisticsTable) {
  if (key === "top_students") return ["Sinh viên", "Mã SV", "Số lần", "Điểm cao nhất", "Trạng thái"];
  if (key === "live_status") return ["Sinh viên", "Mã SV", "Số lần", "Tiến trình", "Cảnh báo"];
  return table.columns;
}

export function buildTeacherRows(exam: TeacherExamDetail | undefined, key: string, table: StatisticsTable) {
  if (key === "top_students" && exam?.students?.length) {
    return exam.students
      .filter((student) => Number.isFinite(scoreValue(student.score)))
      .sort((left, right) => scoreValue(right.score) - scoreValue(left.score))
      .map((student, rowIndex) => ({
        cells: [student.name, student.studentCode, String(student.attemptCount), student.score, student.warning],
        rowIndex,
      }));
  }
  if (key === "live_status") {
    return (exam?.students || []).map((student, rowIndex) => ({
      cells: [student.name, student.studentCode, String(student.attemptCount), student.progress, student.warning],
      rowIndex,
    }));
  }
  return table.rows.map((cells, rowIndex) => ({ cells, rowIndex }));
}

export function renderTeacherModal(exam: TeacherExamDetail | undefined, key: string, table: StatisticsTable, rowIndex: number): ReactNode {
  if (!exam) return <p>Chưa có dữ liệu.</p>;
  if (key === "live_status") return renderStudentAttempt(exam.students?.[rowIndex]);
  if (key === "top_students") {
    const student = (exam.students || [])
      .filter((candidate) => Number.isFinite(scoreValue(candidate.score)))
      .sort((left, right) => scoreValue(right.score) - scoreValue(left.score))[rowIndex];
    return renderStudentAttempt(student);
  }
  if (key === "score_distribution") {
    const row = table.rows[rowIndex];
    const students = studentsForScoreBucket(exam.students || [], row?.[0] || "");
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
    return (
      <>
        <p className="eyebrow">Chi tiết câu hỏi</p>
        <h2>{row[0]} - {row[2]}</h2>
        <div className="student-detail-meta"><span>Tỷ lệ sai: {row[1]}</span><span>Sinh viên liên quan: chưa đủ dữ liệu</span></div>
        <p className="empty-note">Danh sách sinh viên sai câu này sẽ lấy từ bảng câu trả lời thật khi luồng làm bài ghi đầy đủ `student_answers`.</p>
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

function renderStudentAttempt(student?: StudentAttemptDetail): ReactNode {
  if (!student) return <p>Chưa có dữ liệu sinh viên.</p>;
  return <StudentAttemptModalContent student={student} />;
}

function StudentAttemptModalContent({ student }: { student: StudentAttemptDetail }) {
  const attempts = student.attempts || [];
  const [selectedAttemptNo, setSelectedAttemptNo] = useState(attempts[0]?.attemptNo ?? 0);
  const selectedAttempt = useMemo(
    () => attempts.find((attempt) => attempt.attemptNo === selectedAttemptNo) || attempts[0],
    [attempts, selectedAttemptNo],
  );

  if (!attempts.length) {
    return (
      <>
        <p className="eyebrow">Chi tiết sinh viên</p>
        <h2>{student.name}</h2>
        <div className="student-detail-meta student-detail-meta-bright">
          <span>Mã SV: {student.studentCode}</span>
          <span>Số lần làm: {student.attemptCount}</span>
          <span>Điểm cao nhất: {student.score}</span>
          <span>Trạng thái: {student.warning}</span>
        </div>
        <p className="empty-note">Sinh viên này chưa có lần làm để xem lại.</p>
      </>
    );
  }

  return (
    <>
      <p className="eyebrow">Chi tiết sinh viên</p>
      <h2>{student.name}</h2>
      <div className="student-detail-meta student-detail-meta-bright">
        <span>Mã SV: {student.studentCode}</span>
        <span>Số lần làm: {student.attemptCount}</span>
        <span>Điểm cao nhất: {student.score}</span>
        <span>Trạng thái: {student.warning}</span>
      </div>
      <div className="attempt-picker">
        {attempts.map((attempt) => (
          <button
            className={`attempt-tab ${attempt.attemptNo === selectedAttempt?.attemptNo ? "active" : ""}`}
            type="button"
            key={attempt.attemptNo}
            onClick={() => setSelectedAttemptNo(attempt.attemptNo)}
          >
            <strong>Lần {attempt.attemptNo}</strong>
            <span>Điểm {attempt.score}</span>
          </button>
        ))}
      </div>
      {selectedAttempt && (
        <div className="student-attempt-detail">
          <div className="attempt-summary-grid">
            <div className="attempt-summary-card"><span>Lần làm</span><strong>{selectedAttempt.attemptNo}</strong></div>
            <div className="attempt-summary-card"><span>Điểm</span><strong>{selectedAttempt.score}</strong></div>
            <div className="attempt-summary-card"><span>Thời gian</span><strong>{selectedAttempt.duration}</strong></div>
            <div className="attempt-summary-card"><span>Trạng thái</span><strong>{selectedAttempt.status}</strong></div>
            <div className="attempt-summary-card attempt-summary-wide"><span>Nộp lúc</span><strong>{selectedAttempt.submittedAt}</strong></div>
          </div>
          <div className="wrong-list">
            <h3>Câu sai trong lần {selectedAttempt.attemptNo}</h3>
            <AttemptWrongItems items={selectedAttempt.wrongItems || []} />
          </div>
        </div>
      )}
    </>
  );
}

function AttemptWrongItems({ items }: { items: StudentAttemptDetail["wrongItems"] }) {
  if (!items.length) {
    return <p className="empty-note">Chưa có dữ liệu câu trả lời cho lần này.</p>;
  }
  return (
    <div className="attempt-wrong-list">
      {items.map((item) => (
        <div className="attempt-wrong-item" key={`${item.question}-${item.selected}-${item.correct}`}>
          <strong>{item.question}</strong>
          <span>Đã chọn: {item.selected}</span>
          <span>Đáp án đúng: {item.correct}</span>
          <p>{item.note}</p>
        </div>
      ))}
    </div>
  );
}

function StudentPreviewList({ students }: { students: StudentAttemptDetail[] }) {
  if (!students.length) return <p className="empty-note">Chưa có sinh viên phù hợp hàng thống kê này.</p>;
  return (
    <div className="wrong-list">
      {students.map((student) => (
        <div className="wrong-item" key={student.name}>
          <strong>{student.name}</strong>
          <span>Mã SV: {student.studentCode}</span>
          <span>Số lần làm: {student.attemptCount}</span>
          <span>Điểm cao nhất: {student.score}</span>
          <span>Cảnh báo: {student.warning}</span>
        </div>
      ))}
    </div>
  );
}

function studentsForScoreBucket(students: StudentAttemptDetail[], bucket: string) {
  const normalized = bucket.toLowerCase();
  return students.filter((student) => {
    const score = scoreValue(student.score);
    if (!Number.isFinite(score)) return false;
    if (normalized.includes("dưới") || normalized.includes("duoi")) {
      const limit = firstNumber(normalized) ?? 5;
      return score < limit;
    }
    const [min, max] = normalized.match(/\d+(?:[.,]\d+)?/g)?.map((value) => Number(value.replace(",", "."))) || [];
    if (min === undefined || max === undefined) return false;
    return score >= min && score <= max;
  });
}

function scoreValue(value: string) {
  const parsed = Number(String(value).replace(",", ".").match(/\d+(?:\.\d+)?/)?.[0]);
  return Number.isFinite(parsed) ? parsed : Number.NaN;
}

function firstNumber(value: string) {
  const parsed = Number(value.match(/\d+(?:[.,]\d+)?/)?.[0]?.replace(",", "."));
  return Number.isFinite(parsed) ? parsed : undefined;
}
