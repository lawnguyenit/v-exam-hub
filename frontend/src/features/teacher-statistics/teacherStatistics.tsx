import type { ReactNode } from "react";
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

export function buildTeacherRows(exam: TeacherExamDetail | undefined, key: string, table: StatisticsTable) {
  if (key === "live_status") {
    return (exam?.students || []).map((student, rowIndex) => ({ cells: [student.name, student.progress, student.warning], rowIndex }));
  }
  return table.rows.map((cells, rowIndex) => ({ cells, rowIndex }));
}

export function renderTeacherModal(exam: TeacherExamDetail | undefined, key: string, table: StatisticsTable, rowIndex: number): ReactNode {
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

function renderStudentAttempt(student?: StudentAttemptDetail): ReactNode {
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
