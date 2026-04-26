import { type FormEvent, useState } from "react";
import { Link, Navigate, useNavigate } from "react-router-dom";
import { createAdminTeacher, logout as logoutSession, type TeacherCreateResult } from "../../api";
import { useRequiredAuth } from "../../lib/auth";
import { Brand } from "../../shared/Brand";
import { clearAuth } from "../../storage";

const departments = ["Công nghệ thông tin", "Toán", "Kinh tế", "Ngoại ngữ"];

const emptyForm = {
  password: "",
  fullName: "",
  email: "",
  phone: "",
  department: departments[0],
};

export function AdminTeachers() {
  const auth = useRequiredAuth("admin");
  const navigate = useNavigate();
  const [form, setForm] = useState(emptyForm);
  const [message, setMessage] = useState("");
  const [result, setResult] = useState<TeacherCreateResult | null>(null);
  const [isSaving, setIsSaving] = useState(false);

  if (!auth) return <Navigate to="/login/teacher" replace />;
  const admin = auth;

  function updateField(field: keyof typeof emptyForm, value: string) {
    setForm((current) => ({ ...current, [field]: value }));
  }

  async function logout() {
    try {
      await logoutSession();
    } catch {
      // Clear local state even if the server is unreachable.
    }
    clearAuth();
    navigate("/");
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSaving(true);
    setMessage("Đang tạo tài khoản giáo viên...");
    setResult(null);
    try {
      const created = await createAdminTeacher({
        adminUsername: admin.account,
        username: "",
        teacherCode: "",
        password: form.password,
        fullName: form.fullName,
        email: form.email,
        phone: form.phone,
        department: form.department,
      });
      setResult(created);
      setMessage(created.created ? "Đã tạo tài khoản giáo viên." : "Đã cập nhật tài khoản giáo viên.");
      setForm(emptyForm);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Không tạo được tài khoản giáo viên.");
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <div className="admin-shell">
      <header className="teacher-topbar">
        <div className="teacher-nav">
          <Brand to="/admin" />
          <nav className="teacher-nav" aria-label="Điều hướng admin">
            <Link className="nav-tab active" to="/admin">
              Giáo viên
            </Link>
          </nav>
        </div>
        <div className="teacher-account-actions">
          <div className="account-chip">
            <span className="avatar">{(admin.displayName || admin.account).slice(0, 2).toUpperCase()}</span>
            <span>
              <strong>{admin.displayName || admin.account}</strong>
              <small>Admin hệ thống</small>
            </span>
          </div>
          <button className="logout-btn" type="button" onClick={logout}>
            Đăng xuất
          </button>
        </div>
      </header>

      <main className="teacher-layout single-column">
        <section className="hero-row">
          <div>
            <p className="eyebrow">Quản trị tài khoản</p>
            <h1>Tạo tài khoản giáo viên</h1>
            <p>Admin cấp tài khoản giáo viên tại đây, không cần sửa SQL trực tiếp.</p>
          </div>
        </section>

        <section className="admin-panel">
          <form className="admin-teacher-form" onSubmit={submit}>
            <div className="form-grid two">
              <label>
                Họ và tên
                <input value={form.fullName} onChange={(event) => updateField("fullName", event.target.value)} placeholder="VD: Nguyễn Lâm Nguyên" required />
              </label>
              <label>
                Khoa / bộ môn
                <select value={form.department} onChange={(event) => updateField("department", event.target.value)}>
                  {departments.map((department) => (
                    <option key={department} value={department}>
                      {department}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Mật khẩu tạm
                <input value={form.password} onChange={(event) => updateField("password", event.target.value)} placeholder="Bỏ trống sẽ dùng tài khoản" />
              </label>
              <label>
                Email
                <input type="email" value={form.email} onChange={(event) => updateField("email", event.target.value)} placeholder="email@school.edu.vn" />
              </label>
              <label>
                Số điện thoại
                <input value={form.phone} onChange={(event) => updateField("phone", event.target.value)} placeholder="09..." />
              </label>
            </div>
            <p className="form-hint">Mã giáo viên tự sinh từ họ tên. Ví dụ: Nguyễn Lâm Nguyên thành nguyennl; nếu trùng hệ thống tự thêm số.</p>
            <button className="primary-btn" type="submit" disabled={isSaving}>
              {isSaving ? "Đang lưu..." : "Tạo tài khoản giáo viên"}
            </button>
          </form>

          {message && <p className="form-message">{message}</p>}
          {result && (
            <div className="admin-result">
              <p className="eyebrow">Tài khoản đã cấp</p>
              <div className="result-grid">
                <span>Tài khoản</span>
                <strong>{result.username}</strong>
                <span>Mật khẩu tạm</span>
                <strong>{result.temporaryPassword}</strong>
                <span>Mã giáo viên</span>
                <strong>{result.teacherCode}</strong>
                <span>Họ tên</span>
                <strong>{result.fullName}</strong>
                <span>Khoa / bộ môn</span>
                <strong>{result.department || "-"}</strong>
              </div>
            </div>
          )}
        </section>
      </main>
    </div>
  );
}
