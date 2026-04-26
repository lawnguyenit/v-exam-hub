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
          <Link className="nav-tab" to="/teacher/question-bank">Đề cương</Link>
          <Link className="nav-tab" to="/teacher/classes">Lớp</Link>
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
  type ImportedStudent = StudentImportResult["importedStudents"][number];
  type RowError = NonNullable<StudentImportResult["rowErrors"]>[number];
  const legacyErrors: RowError[] = (result.errors ?? []).map((message, index) => ({
    sourceRow: index + 1,
    studentCode: "",
    fullName: "",
    email: "",
    phone: "",
    username: "",
    password: "",
    message,
  }));
  const [students, setStudents] = useState<ImportedStudent[]>(() => result.importedStudents ?? []);
  const [rowErrors, setRowErrors] = useState<RowError[]>(() => result.rowErrors?.length ? result.rowErrors : legacyErrors);
  const [stats, setStats] = useState({
    created: result.created,
    updated: result.updated,
    addedToClass: result.addedToClass,
    skipped: result.skipped,
  });
  const [editingRows, setEditingRows] = useState<Record<number, RowError>>({});
  const [passwords, setPasswords] = useState<Record<string, string>>({});
  const [visiblePasswords, setVisiblePasswords] = useState<Record<string, string>>(() => Object.fromEntries(
    (result.importedStudents ?? []).map((student) => [student.studentCode, student.temporaryPassword]),
  ));
  const [saving, setSaving] = useState("");
  const [savingRow, setSavingRow] = useState(0);
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
      ["Dòng XLSX", "Mã SV", "Họ tên", "Tài khoản", "Mật khẩu"],
      ...students.map((student) => [
        student.sourceRow ? String(student.sourceRow) : "",
        student.studentCode,
        student.fullName,
        student.username,
        visiblePasswords[student.studentCode] || student.temporaryPassword || "",
      ]),
    ];
    const blob = createXLSXBlob(lines, "Tai khoan");
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `tai-khoan-${safeFileName(result.classCode)}.xlsx`;
    link.click();
    URL.revokeObjectURL(url);
  }

  function startEditRow(row: RowError) {
    setEditingRows((current) => ({ ...current, [row.sourceRow]: { ...row } }));
  }

  function cancelEditRow(sourceRow: number) {
    setEditingRows((current) => {
      const next = { ...current };
      delete next[sourceRow];
      return next;
    });
  }

  function updateEditingRow(sourceRow: number, field: Exclude<keyof RowError, "sourceRow">, value: string) {
    setEditingRows((current) => ({
      ...current,
      [sourceRow]: { ...current[sourceRow], [field]: value },
    }));
  }

  async function saveMissingRow(row: RowError) {
    const draft = editingRows[row.sourceRow] || row;
    if (!draft.studentCode.trim() || !draft.fullName.trim()) {
      setMessage(`Dòng ${row.sourceRow} cần có mã sinh viên và họ tên trước khi lưu.`);
      return;
    }
    setSavingRow(row.sourceRow);
    setMessage(`Đang lưu lại dòng ${row.sourceRow}...`);
    try {
      const rows = [
        manualTemplate.trim(),
        [draft.studentCode, draft.fullName, draft.email, draft.phone, draft.username, draft.password].map(csvCell).join(","),
      ].join("\n");
      const response = await importTeacherClassStudents({ classCode: result.classCode, className: result.className, rows });
      const imported = (response.importedStudents ?? []).map((student) => ({ ...student, sourceRow: row.sourceRow }));
      if (imported.length === 0) {
        setMessage(response.errors?.[0] || `Dòng ${row.sourceRow} vẫn chưa đủ dữ liệu để import.`);
        return;
      }
      setStudents((current) => {
        const byCode = new Map(current.map((student) => [student.studentCode, student]));
        imported.forEach((student) => byCode.set(student.studentCode, student));
        return Array.from(byCode.values()).sort((left, right) => (left.sourceRow || 0) - (right.sourceRow || 0));
      });
      setVisiblePasswords((current) => ({
        ...current,
        ...Object.fromEntries(imported.map((student) => [student.studentCode, student.temporaryPassword])),
      }));
      setStats((current) => ({
        created: current.created + response.created,
        updated: current.updated + response.updated,
        addedToClass: current.addedToClass + response.addedToClass,
        skipped: Math.max(0, current.skipped - 1 + response.skipped),
      }));
      setRowErrors((current) => current.filter((item) => item.sourceRow !== row.sourceRow));
      cancelEditRow(row.sourceRow);
      setMessage(`Đã import lại dòng ${row.sourceRow}.`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : `Không lưu được dòng ${row.sourceRow}.`);
    } finally {
      setSavingRow(0);
    }
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
            <button className="ghost-btn" type="button" onClick={exportAccounts}>Export XLSX</button>
            <button className="ghost-btn" type="button" onClick={onClose}>Đóng</button>
          </div>
        </header>

        <div className="student-import-stats">
          <span><strong>{stats.created}</strong>Tạo mới</span>
          <span><strong>{stats.updated}</strong>Cập nhật</span>
          <span><strong>{stats.addedToClass}</strong>Trong lớp</span>
          <span><strong>{stats.skipped}</strong>Bỏ qua</span>
        </div>

        <p className="form-note">{message}</p>

        <div className="password-table">
          <div className="password-row password-row-edit header">
            <span>Dòng</span><span>Mã SV</span><span>Họ tên</span><span>Tài khoản</span><span>Mật khẩu tạm</span><span>Đổi mật khẩu</span>
          </div>
          {students.length === 0 && <p className="parser-empty">Không có tài khoản sinh viên nào được tạo từ file này.</p>}
          {students.map((student) => (
            <div className="password-row password-row-edit" key={student.studentCode}>
              <span>{student.sourceRow || "--"}</span>
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

        {rowErrors.length > 0 && (
          <div className="import-errors">
            <h2>Dòng cần kiểm tra</h2>
            {rowErrors.map((error) => {
              const draft = editingRows[error.sourceRow];
              return (
                <div className="student-row-error" key={`${error.sourceRow}-${error.message}`}>
                  <div className="student-row-error-head">
                    <strong>Dòng XLSX {error.sourceRow}</strong>
                    <span>{error.message}</span>
                    <div className="modal-actions">
                      {draft ? (
                        <>
                          <button className="ghost-btn" type="button" onClick={() => cancelEditRow(error.sourceRow)} disabled={savingRow === error.sourceRow}>Hủy</button>
                          <button className="ghost-btn" type="button" onClick={() => saveMissingRow(error)} disabled={savingRow === error.sourceRow}>
                            {savingRow === error.sourceRow ? "Đang lưu" : "Lưu"}
                          </button>
                        </>
                      ) : (
                        <button className="ghost-btn" type="button" onClick={() => startEditRow(error)}>Sửa</button>
                      )}
                    </div>
                  </div>
                  {draft ? (
                    <div className="student-row-error-grid">
                      <input value={draft.studentCode} onChange={(event) => updateEditingRow(error.sourceRow, "studentCode", event.target.value)} placeholder="Mã SV" />
                      <input value={draft.fullName} onChange={(event) => updateEditingRow(error.sourceRow, "fullName", event.target.value)} placeholder="Họ tên" />
                      <input value={draft.email} onChange={(event) => updateEditingRow(error.sourceRow, "email", event.target.value)} placeholder="Email" />
                      <input value={draft.phone} onChange={(event) => updateEditingRow(error.sourceRow, "phone", event.target.value)} placeholder="SĐT" />
                      <input value={draft.username} onChange={(event) => updateEditingRow(error.sourceRow, "username", event.target.value)} placeholder="Tài khoản, có thể trống" />
                      <input value={draft.password} onChange={(event) => updateEditingRow(error.sourceRow, "password", event.target.value)} placeholder="Mật khẩu, có thể trống" />
                    </div>
                  ) : (
                    <p>{[error.studentCode || "thiếu mã SV", error.fullName || "thiếu họ tên", error.email, error.phone].filter(Boolean).join(" - ")}</p>
                  )}
                </div>
              );
            })}
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

function safeFileName(value: string) {
  return value.trim().replace(/[\\/:*?"<>|]+/g, "-").replace(/\s+/g, "-") || "lop";
}

function createXLSXBlob(rows: string[][], sheetName: string) {
  const files: Record<string, string> = {
    "[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`,
    "_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`,
    "xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="${xmlEscape(sheetName)}" sheetId="1" r:id="rId1"/></sheets>
</workbook>`,
    "xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`,
    "xl/styles.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts>
<fills count="3"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill><fill><patternFill patternType="solid"><fgColor rgb="FFEAF3ED"/><bgColor indexed="64"/></patternFill></fill></fills>
<borders count="2"><border><left/><right/><top/><bottom/><diagonal/></border><border><left style="thin"><color rgb="FFD8E3DC"/></left><right style="thin"><color rgb="FFD8E3DC"/></right><top style="thin"><color rgb="FFD8E3DC"/></top><bottom style="thin"><color rgb="FFD8E3DC"/></bottom><diagonal/></border></borders>
<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
<cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="1" xfId="0"/><xf numFmtId="0" fontId="1" fillId="2" borderId="1" xfId="0" applyFont="1" applyFill="1"/><xf numFmtId="0" fontId="1" fillId="0" borderId="1" xfId="0" applyFont="1"/></cellXfs>
</styleSheet>`,
    "xl/worksheets/sheet1.xml": createWorksheetXML(rows),
  };
  return new Blob([zipStore(files)], { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" });
}

function createWorksheetXML(rows: string[][]) {
  const maxColumns = rows.reduce((max, row) => Math.max(max, row.length), 0);
  const refs = `A1:${columnName(maxColumns)}${Math.max(rows.length, 1)}`;
  const body = rows.map((row, rowIndex) => {
    const cells = row.map((value, columnIndex) => {
      const ref = `${columnName(columnIndex + 1)}${rowIndex + 1}`;
      const style = rowIndex === 0 ? 1 : columnIndex === 4 ? 2 : 0;
      return `<c r="${ref}" t="inlineStr" s="${style}"><is><t>${xmlEscape(value)}</t></is></c>`;
    }).join("");
    return `<row r="${rowIndex + 1}">${cells}</row>`;
  }).join("");
  return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<dimension ref="${refs}"/>
<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" activePane="bottomLeft" state="frozen"/></sheetView></sheetViews>
<cols><col min="1" max="1" width="12" customWidth="1"/><col min="2" max="2" width="16" customWidth="1"/><col min="3" max="3" width="28" customWidth="1"/><col min="4" max="4" width="18" customWidth="1"/><col min="5" max="5" width="18" customWidth="1"/></cols>
<sheetData>${body}</sheetData>
<autoFilter ref="${refs}"/>
</worksheet>`;
}

function zipStore(files: Record<string, string>) {
  const encoder = new TextEncoder();
  const chunks: Uint8Array[] = [];
  const central: Uint8Array[] = [];
  let offset = 0;
  for (const [name, content] of Object.entries(files)) {
    const nameBytes = encoder.encode(name);
    const data = encoder.encode(content);
    const crc = crc32(data);
    const local = new Uint8Array(30 + nameBytes.length);
    const localView = new DataView(local.buffer);
    localView.setUint32(0, 0x04034b50, true);
    localView.setUint16(4, 20, true);
    localView.setUint16(8, 0, true);
    localView.setUint32(14, crc, true);
    localView.setUint32(18, data.length, true);
    localView.setUint32(22, data.length, true);
    localView.setUint16(26, nameBytes.length, true);
    local.set(nameBytes, 30);
    chunks.push(local, data);

    const entry = new Uint8Array(46 + nameBytes.length);
    const entryView = new DataView(entry.buffer);
    entryView.setUint32(0, 0x02014b50, true);
    entryView.setUint16(4, 20, true);
    entryView.setUint16(6, 20, true);
    entryView.setUint32(16, crc, true);
    entryView.setUint32(20, data.length, true);
    entryView.setUint32(24, data.length, true);
    entryView.setUint16(28, nameBytes.length, true);
    entryView.setUint32(42, offset, true);
    entry.set(nameBytes, 46);
    central.push(entry);
    offset += local.length + data.length;
  }
  const centralSize = central.reduce((sum, item) => sum + item.length, 0);
  const end = new Uint8Array(22);
  const endView = new DataView(end.buffer);
  endView.setUint32(0, 0x06054b50, true);
  endView.setUint16(8, central.length, true);
  endView.setUint16(10, central.length, true);
  endView.setUint32(12, centralSize, true);
  endView.setUint32(16, offset, true);
  return concatUint8Arrays([...chunks, ...central, end]);
}

function concatUint8Arrays(parts: Uint8Array[]) {
  const out = new Uint8Array(parts.reduce((sum, part) => sum + part.length, 0));
  let offset = 0;
  for (const part of parts) {
    out.set(part, offset);
    offset += part.length;
  }
  return out;
}

function crc32(data: Uint8Array) {
  let crc = 0xffffffff;
  for (const byte of data) {
    crc ^= byte;
    for (let index = 0; index < 8; index++) {
      crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1));
    }
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function columnName(index: number) {
  let name = "";
  while (index > 0) {
    index--;
    name = String.fromCharCode(65 + (index % 26)) + name;
    index = Math.floor(index / 26);
  }
  return name || "A";
}

function xmlEscape(value: string) {
  return String(value ?? "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}
