import { type FormEvent, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { type Role, writeAuth } from "../../storage";
import { Brand } from "../../shared/Brand";

export function LoginPage({ role }: { role: Role }) {
  const navigate = useNavigate();
  const [account, setAccount] = useState("");
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState(
    role === "teacher"
      ? "Luồng giáo viên hiện là placeholder, nhưng vẫn đi qua bước đăng nhập để giữ đúng flow."
      : "Bản UI test chấp nhận tài khoản/mật khẩu bất kỳ. Backend thật sẽ xác thực bằng dữ liệu trường cấp.",
  );

  useEffect(() => {
    sessionStorage.setItem("examhub:lastRole", role);
  }, [role]);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!account.trim() || !password.trim()) {
      setMessage("Cần nhập đủ tài khoản và mật khẩu được cấp.");
      return;
    }
    writeAuth({ account: account.trim(), role, signedInAt: Date.now() });
    navigate(role === "teacher" ? "/teacher" : "/student");
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
          <button className="primary-btn" type="submit">Vào dashboard</button>
          <p className="form-message">{message}</p>
        </form>
        <Link className="back-link" to="/">Chọn tư cách khác</Link>
      </section>
    </main>
  );
}
