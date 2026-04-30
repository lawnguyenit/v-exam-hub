import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { getCurrentSession } from "../../api";
import { Brand } from "../../shared/Brand";
import { clearAuth, writeAuth, type Role } from "../../lib/authStorage";

function dashboardFor(role: Role) {
  if (role === "admin") return "/admin";
  if (role === "teacher") return "/teacher";
  return "/student";
}

export function RoleSelect() {
  const navigate = useNavigate();
  const [isRestoring, setIsRestoring] = useState(true);

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
        if (cancelled) return;
        clearAuth();
        setIsRestoring(false);
      });

    return () => {
      cancelled = true;
    };
  }, [navigate]);

  return (
    <main className="login-page login-shell" aria-label="Chọn tư cách đăng nhập">
      <section className="login-box role-box">
        <Brand />
        <div className="login-copy">
          <p className="eyebrow">Tài khoản do admin cấp</p>
          <h1>Chọn tư cách đăng nhập</h1>
        </div>
        <nav className="login-bars" aria-label="Vai trò đăng nhập">
          <Link className="login-bar teacher" to="/login/teacher">
            <span>Đăng nhập với tư cách giáo viên</span>
            <strong>Quản lý lớp thi</strong>
          </Link>
          <Link className="login-bar student" to="/login/student">
            <span>Đăng nhập với tư cách sinh viên / học sinh</span>
            <strong>Vào dashboard làm bài</strong>
          </Link>
        </nav>
        {isRestoring && <p className="form-message">Đang kiểm tra phiên đăng nhập...</p>}
      </section>
    </main>
  );
}


