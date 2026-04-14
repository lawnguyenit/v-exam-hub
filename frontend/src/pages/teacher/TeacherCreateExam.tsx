import type { FormEvent } from "react";
import { useState } from "react";
import { Navigate } from "react-router-dom";
import { useRequiredAuth } from "../../lib/auth";
import { PageShell } from "../../shared/PageShell";

export function TeacherCreateExam() {
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
