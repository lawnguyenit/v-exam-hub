import { type FormEvent, useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { ApiError, getCurrentSession, login } from "../../api";
import { Brand } from "../../shared/Brand";
import { type Role, clearAuth, writeAuth } from "../../storage";

const roleCopy: Record<Role, { eyebrow: string; title: string; description: string; placeholder: string; password: string; message: string }> = {
  admin: {
    eyebrow: "Admin",
    title: "Quản trị tài khoản",
    description: "Dùng tài khoản admin để cấp và quản lý tài khoản giáo viên.",
    placeholder: "VD: root",
    password: "Mật khẩu admin",
    message: "Chỉ tài khoản admin mới được cấp tài khoản giáo viên.",
  },
  teacher: {
    eyebrow: "Giáo viên",
    title: "Đăng nhập quản lý lớp thi",
    description: "Tài khoản giáo viên do admin cấp.",
    placeholder: "VD: gv-cntt-01",
    password: "Mật khẩu giáo viên",
    message: "Chỉ tài khoản giáo viên đã được cấp mới đăng nhập được.",
  },
  student: {
    eyebrow: "Sinh viên / học sinh",
    title: "Đăng nhập để làm bài",
    description: "Tiến trình cũ sẽ được mở lại nếu đăng nhập cùng tài khoản trên trình duyệt này.",
    placeholder: "VD: mã sinh viên hoặc email",
    password: "Mật khẩu được cấp",
    message: "Chỉ tài khoản sinh viên/học sinh đã được cấp mới đăng nhập được.",
  },
};

function dashboardFor(role: Role) {
  if (role === "admin") return "/admin";
  if (role === "teacher") return "/teacher";
  return "/student";
}

export function LoginPage({ role }: { role: Role }) {
  const navigate = useNavigate();
  const copy = useMemo(() => roleCopy[role], [role]);
  const [account, setAccount] = useState("");
  const [password, setPassword] = useState("");
  const [isSigningIn, setIsSigningIn] = useState(false);
  const [message, setMessage] = useState(copy.message);
  const [sessionConflict, setSessionConflict] = useState("");

  useEffect(() => {
    sessionStorage.setItem("examhub:lastRole", role);
    setMessage(copy.message);
  }, [copy.message, role]);

  useEffect(() => {
    let cancelled = false;
    getCurrentSession()
      .then((auth) => {
        if (cancelled) return;
        writeAuth({
          account: auth.username,
          role: auth.role,
          signedInAt: Date.now(),
          displayName: auth.displayName,
        });
        navigate(dashboardFor(auth.role), { replace: true });
      })
      .catch(() => {
        if (!cancelled) clearAuth();
      });

    return () => {
      cancelled = true;
    };
  }, [navigate]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!account.trim() || !password.trim()) {
      setMessage("Cần nhập đủ tài khoản và mật khẩu được cấp.");
      return;
    }
    setIsSigningIn(true);
    setMessage("Đang xác thực tài khoản...");
    setSessionConflict("");
    try {
      const auth = await login({ username: account.trim(), password, role });
      writeAuth({ account: auth.username, role: auth.role, signedInAt: Date.now(), displayName: auth.displayName });
      navigate(dashboardFor(auth.role));
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        setSessionConflict("Tài khoản này đang có phiên đăng nhập khác. Hãy đăng xuất ở phiên cũ hoặc thử lại sau.");
        setMessage(copy.message);
      } else {
        setMessage(error instanceof Error ? error.message : "Đăng nhập thất bại.");
      }
    } finally {
      setIsSigningIn(false);
    }
  }

  return (
    <main className="login-page login-shell" aria-label={`Đăng nhập ${copy.eyebrow}`}>
      <section className="login-box form-box">
        <Brand />
        <form className="credential-form standalone" onSubmit={submit}>
          <div className="form-head">
            <p className="eyebrow">{copy.eyebrow}</p>
            <h1>{copy.title}</h1>
            <p>{copy.description}</p>
          </div>
          <label htmlFor="accountInput">Tài khoản</label>
          <input
            id="accountInput"
            value={account}
            onChange={(event) => setAccount(event.target.value)}
            autoComplete="username"
            placeholder={copy.placeholder}
            required
          />
          <label htmlFor="passwordInput">Mật khẩu</label>
          <input
            id="passwordInput"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete="current-password"
            placeholder={copy.password}
            required
          />
          <button className="primary-btn" type="submit" disabled={isSigningIn}>
            {isSigningIn ? "Đang đăng nhập..." : "Vào dashboard"}
          </button>
          <p className="form-message">{message}</p>
        </form>
        <Link className="back-link" to="/">Chọn tư cách khác</Link>
      </section>

      {sessionConflict && (
        <div className="login-modal-backdrop" role="presentation">
          <section className="login-modal" role="dialog" aria-modal="true" aria-labelledby="sessionConflictTitle">
            <p className="eyebrow">Phiên đăng nhập</p>
            <h2 id="sessionConflictTitle">Tài khoản đang được dùng</h2>
            <p>{sessionConflict}</p>
            <button className="primary-btn" type="button" onClick={() => setSessionConflict("")}>
              Đã hiểu
            </button>
          </section>
        </div>
      )}
    </main>
  );
}
