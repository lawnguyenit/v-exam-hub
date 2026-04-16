import type { FormEvent } from "react";
import { useState } from "react";
import { Link, Navigate } from "react-router-dom";
import { importTeacherClassStudents, importTeacherClassStudentsFile, type StudentImportResult, updateTeacherStudentPassword } from "../../api";
import { useRequiredAuth } from "../../lib/auth";
import { Brand } from "../../shared/Brand";

const manualTemplate = "Mã SV,Họ tên,Email,SĐT,Tài khoản,Mật khẩu\n";

export function TeacherStudents() {
  const auth = useRequiredAuth("teacher");
  const [mode, setMode] = useState<"xlsx" | "manual">("xlsx");
  const [classCode, setClassCode] = useState("CNTT K48");
  const [className, setClassName] = useState("Công nghệ thông tin K48");
  const [rows, setRows] = useState(manualTemplate);
  const [quickStudent, setQuickStudent] = useState({ studentCode: "", fullName: "", email: "", phone: "", username: "", password: "" });
  const [file, setFile] = useState<File | undefined>();
  const [isImporting, setIsImporting] = useState(false);
  const [result, setResult] = useState<StudentImportResult | undefined>();
  const [message, setMessage] = useState("Chọn file XLSX danh sách lớp hoặc chuyển sang nhập thủ công. Mật khẩu có thể để trống để hệ thống tự tạo.");

  if (!auth) return <Navigate to="/" replace />;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsImporting(true);
    setResult(undefined);
    setMessage("Đang tạo tài khoản và gắn sinh viên vào lớp...");
    try {
      if (mode === "xlsx" && !file) {
        setMessage("Cần chọn file XLSX trước khi import.");
        return;
      }
      const response = mode === "xlsx" && file
        ? await importTeacherClassStudentsFile({ classCode, className, file })
        : await importTeacherClassStudents({ classCode, className, rows });
      setResult(response);
      setMessage(`Đã xử lý lớp ${response.classCode}.`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Không import được danh sách sinh viên.");
    } finally {
      setIsImporting(false);
    }
  }

  function updateQuickStudent(field: keyof typeof quickStudent, value: string) {
    setQuickStudent((current) => ({ ...current, [field]: value }));
  }

  function addQuickStudent() {
    if (!quickStudent.studentCode.trim() || !quickStudent.fullName.trim()) {
      setMessage("Thêm nhanh cần ít nhất mã sinh viên và họ tên.");
      return;
    }
    const line = [
      quickStudent.studentCode,
      quickStudent.fullName,
      quickStudent.email,
      quickStudent.phone,
      quickStudent.username,
      quickStudent.password,
    ].map(csvCell).join(",");
    setRows((current) => {
      const trimmed = current.trim();
      if (!trimmed) return `Mã SV,Họ tên,Email,SĐT,Tài khoản,Mật khẩu\n${line}`;
      return `${trimmed}\n${line}`;
    });
    setQuickStudent({ studentCode: "", fullName: "", email: "", phone: "", username: "", password: "" });
    setMessage("Đã thêm một dòng vào danh sách nhập thủ công.");
  }

  return (
    <>
      <header className="teacher-topbar">
        <Brand />
        <nav className="teacher-nav" aria-label="Điều hướng giáo viên">
          <Link className="nav-tab" to="/teacher">Dashboard</Link>
          <Link className="nav-tab" to="/teacher/create">Tạo bài kiểm tra</Link>
          <Link className="nav-tab active" to="/teacher/students">Sinh viên</Link>
        </nav>
        <div className="account-chip">
          <span className="avatar">{auth.account.slice(0, 2).toUpperCase()}</span>
          <div>
            <strong>Giáo viên</strong>
            <small>{auth.account}</small>
          </div>
        </div>
      </header>

      <main className="teacher-dashboard teacher-dashboard-single">
        <section className="teacher-workspace">
          <div className="dashboard-hero">
            <div>
              <p className="eyebrow">Quản lý lớp</p>
              <h1>Import tài khoản sinh viên</h1>
              <p className="lead">Tạo tài khoản theo danh sách lớp, nhóm học thêm hoặc nhóm vãng lai; không mở đăng ký tự do đại trà.</p>
            </div>
          </div>

          <form className="student-import-panel" onSubmit={submit}>
            <div className="student-import-tabs" role="tablist" aria-label="Kiểu nhập sinh viên">
              <button className={mode === "xlsx" ? "active" : ""} type="button" onClick={() => setMode("xlsx")}>Import XLSX</button>
              <button className={mode === "manual" ? "active" : ""} type="button" onClick={() => setMode("manual")}>Nhập/dán thủ công</button>
            </div>

            <div className="student-import-grid">
              <label>
                <span className="label-with-tooltip">
                  Mã lớp / nhóm
                  <span className="policy-help" tabIndex={0}>
                    ?
                    <span className="policy-tooltip">
                      Với vãng lai hoặc học thêm, tạo nhóm riêng như VANG-LAI hoặc LOP-THAY-A. Không mở đăng ký tự do; nếu cần đăng ký công khai thì dùng mã mời và giáo viên duyệt.
                    </span>
                  </span>
                </span>
                <input value={classCode} onChange={(event) => setClassCode(event.target.value)} required />
              </label>
              <label>
                Tên lớp / nhóm
                <input value={className} onChange={(event) => setClassName(event.target.value)} required />
              </label>
            </div>

            {mode === "xlsx" ? (
              <label>
                File danh sách lớp XLSX
                <input type="file" accept=".xlsx" onChange={(event) => setFile(event.target.files?.[0])} required={mode === "xlsx"} />
              </label>
            ) : (
              <>
                <div className="quick-student-panel">
                  <strong>Thêm nhanh một sinh viên</strong>
                  <div className="quick-student-grid">
                    <input value={quickStudent.studentCode} onChange={(event) => updateQuickStudent("studentCode", event.target.value)} placeholder="Mã SV" />
                    <input value={quickStudent.fullName} onChange={(event) => updateQuickStudent("fullName", event.target.value)} placeholder="Họ tên" />
                    <input value={quickStudent.email} onChange={(event) => updateQuickStudent("email", event.target.value)} placeholder="Email" />
                    <input value={quickStudent.phone} onChange={(event) => updateQuickStudent("phone", event.target.value)} placeholder="SĐT" />
                    <input value={quickStudent.username} onChange={(event) => updateQuickStudent("username", event.target.value)} placeholder="Tài khoản, có thể trống" />
                    <input value={quickStudent.password} onChange={(event) => updateQuickStudent("password", event.target.value)} placeholder="Mật khẩu, có thể trống" />
                  </div>
                  <button className="ghost-btn" type="button" onClick={addQuickStudent}>Thêm vào danh sách</button>
                </div>

                <label>
                  Danh sách sinh viên
                  <textarea
                    value={rows}
                    onChange={(event) => setRows(event.target.value)}
                    rows={10}
                    placeholder="Mã SV,Họ tên,Email,SĐT,Tài khoản,Mật khẩu"
                    required={mode === "manual"}
                  />
                </label>
              </>
            )}

            <p className="form-note">
              XLSX nhận các cột như MÃ SV, HỌ VÀ TÊN, Mã lớp, Email, SĐT. Nhập tay nhận: mã sinh viên, họ tên, email, số điện thoại, tài khoản, mật khẩu.
            </p>

            <button className="primary-btn" type="submit" disabled={isImporting}>
              {isImporting ? "Đang import..." : mode === "xlsx" ? "Import từ XLSX" : "Tạo tài khoản sinh viên"}
            </button>
          </form>
          <p className="student-import-message">{message}</p>
        </section>
      </main>
      {result && <StudentImportResultModal result={result} onClose={() => setResult(undefined)} />}
    </>
  );
}

function StudentImportResultModal({ result, onClose }: { result: StudentImportResult; onClose: () => void }) {
  const [passwords, setPasswords] = useState<Record<string, string>>({});
  const [visiblePasswords, setVisiblePasswords] = useState<Record<string, string>>(() => Object.fromEntries(
    result.importedStudents.map((student) => [student.studentCode, student.temporaryPassword]),
  ));
  const [saving, setSaving] = useState("");
  const [message, setMessage] = useState("Có thể đổi mật khẩu từng sinh viên ngay tại đây nếu cần.");

  async function savePassword(student: StudentImportResult["importedStudents"][number]) {
    const password = (passwords[student.studentCode] || "").trim();
    if (!password) {
      setMessage("Cần nhập mật khẩu mới trước khi lưu.");
      return;
    }
    setSaving(student.studentCode);
    try {
      await updateTeacherStudentPassword({ username: student.username, studentCode: student.studentCode, password });
      setMessage(`Đã đổi mật khẩu cho ${student.fullName}.`);
      setVisiblePasswords((current) => ({ ...current, [student.studentCode]: password }));
      setPasswords((current) => ({ ...current, [student.studentCode]: "" }));
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Không đổi được mật khẩu.");
    } finally {
      setSaving("");
    }
  }

  function exportAccounts() {
    const lines = [
      ["Mã SV", "Họ tên", "Tài khoản", "Mật khẩu"],
      ...result.importedStudents.map((student) => [
        student.studentCode,
        student.fullName,
        student.username,
        visiblePasswords[student.studentCode] || student.temporaryPassword || "",
      ]),
    ];
    const csv = lines.map((line) => line.map(csvCell).join(",")).join("\n");
    const blob = new Blob([`\uFEFF${csv}`], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `tai-khoan-${result.classCode}.csv`;
    link.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div className="student-result-backdrop" role="presentation">
      <section className="student-result-modal" role="dialog" aria-modal="true" aria-label="Kết quả import sinh viên">
        <header>
          <div>
            <p className="eyebrow">Đã xử lý danh sách</p>
            <h2>{result.className}</h2>
          </div>
          <div className="modal-actions">
            <button className="ghost-btn" type="button" onClick={exportAccounts}>Export CSV</button>
            <button className="ghost-btn" type="button" onClick={onClose}>Đóng</button>
          </div>
        </header>

        <div className="student-import-stats">
          <span><strong>{result.created}</strong>Tạo mới</span>
          <span><strong>{result.updated}</strong>Cập nhật</span>
          <span><strong>{result.addedToClass}</strong>Trong lớp</span>
          <span><strong>{result.skipped}</strong>Bỏ qua</span>
        </div>

        <p className="form-note">{message}</p>

        <div className="password-table">
          <div className="password-row password-row-edit header">
            <span>Mã SV</span><span>Họ tên</span><span>Tài khoản</span><span>Mật khẩu tạm</span><span>Đổi mật khẩu</span>
          </div>
          {result.importedStudents.map((student) => (
            <div className="password-row password-row-edit" key={student.studentCode}>
              <span>{student.studentCode}</span>
              <span>{student.fullName}</span>
              <span>{student.username}</span>
              <strong>{visiblePasswords[student.studentCode] || student.temporaryPassword || "Chưa cấp"}</strong>
              <span className="password-edit-cell">
                <input
                  value={passwords[student.studentCode] || ""}
                  onChange={(event) => setPasswords((current) => ({ ...current, [student.studentCode]: event.target.value }))}
                  placeholder="Mật khẩu mới"
                />
                <button className="ghost-btn" type="button" onClick={() => savePassword(student)} disabled={saving === student.studentCode}>
                  {saving === student.studentCode ? "Đang lưu" : "Lưu"}
                </button>
              </span>
            </div>
          ))}
        </div>

        {result.errors.length > 0 && (
          <div className="import-errors">
            <h2>Dòng cần kiểm tra</h2>
            {result.errors.map((error) => <p key={error}>{error}</p>)}
          </div>
        )}
      </section>
    </div>
  );
}

function csvCell(value: string) {
  const trimmed = value.trim();
  if (!/[",\n]/.test(trimmed)) return trimmed;
  return `"${trimmed.replace(/"/g, '""')}"`;
}
