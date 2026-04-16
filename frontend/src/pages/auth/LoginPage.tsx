import { type FormEvent, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { login } from "../../api";
import { type Role, writeAuth } from "../../storage";
import { Brand } from "../../shared/Brand";

export function LoginPage({ role }: { role: Role }) {
  const navigate = useNavigate();
  const [account, setAccount] = useState("");
  const [password, setPassword] = useState("");
  const [isSigningIn, setIsSigningIn] = useState(false);
  const [message, setMessage] = useState(
    role === "teacher"
      ? "Chỉ tài khoản giáo viên đã được cấp mới đăng nhập được."
      : "Chỉ tài khoản sinh viên/học sinh đã được cấp mới đăng nhập được.",
  );

  useEffect(() => {
    sessionStorage.setItem("examhub:lastRole", role);
  }, [role]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!account.trim() || !password.trim()) {
      setMessage("Cần nhập đủ tài khoản và mật khẩu được cấp.");
      return;
    }
    setIsSigningIn(true);
    setMessage("Đang xác thực tài khoản...");
    try {
      const auth = await login({ username: account.trim(), password, role });
      writeAuth({ account: auth.username, role, signedInAt: Date.now(), displayName: auth.displayName });
      navigate(role === "teacher" ? "/teacher" : "/student");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Đăng nhập thất bại.");
    } finally {
      setIsSigningIn(false);
    }
  }

  return (
    <main className="login-page login-shell" aria-label={`Đăng nhập ${role === "teacher" ? "giáo viên" : "sinh viên"}`}>
      <section className="login-box form-box">
        <Brand />
        <form className="credential-form standalone" onSubmit={submit}>
          <div className="form-head">
            <p className="eyebrow">{role === "teacher" ? "Giáo viên" : "Sinh viên / học sinh"}</p>
            <h1>{role === "teacher" ? "Đăng nhập quản lý lớp thi" : "Đăng nhập để làm bài"}</h1>
            <p>
              {role === "teacher"
                ? "Tài khoản giáo viên do nhà trường cấp, không mở đăng ký tự do."
                : "Tiến trình cũ sẽ được mở lại nếu đăng nhập cùng tài khoản trên trình duyệt này."}
            </p>
          </div>
          <label htmlFor="accountInput">Tài khoản</label>
          <input
            id="accountInput"
            value={account}
            onChange={(event) => setAccount(event.target.value)}
            autoComplete="username"
            placeholder={role === "teacher" ? "VD: gv-cntt-01" : "VD: lnit hoặc lawnguyenit"}
            required
          />
          <label htmlFor="passwordInput">Mật khẩu</label>
          <input
            id="passwordInput"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete="current-password"
            placeholder={role === "teacher" ? "Mật khẩu giáo viên" : "Mật khẩu hoặc mã truy cập"}
            required
          />
          <button className="primary-btn" type="submit" disabled={isSigningIn}>
            {isSigningIn ? "Đang đăng nhập..." : "Vào dashboard"}
          </button>
          <p className="form-message">{message}</p>
        </form>
        <Link className="back-link" to="/">Chọn tư cách khác</Link>
      </section>
    </main>
  );
}
