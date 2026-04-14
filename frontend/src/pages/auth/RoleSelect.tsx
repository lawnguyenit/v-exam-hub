import { Link } from "react-router-dom";
import { Brand } from "../../shared/Brand";

export function RoleSelect() {
  return (
    <main className="login-page login-shell" aria-label="Chọn tư cách đăng nhập">
      <section className="login-box role-box">
        <Brand />
        <div className="login-copy">
          <p className="eyebrow">Tài khoản do trường cấp</p>
          <h1>Chọn tư cách đăng nhập</h1>
          <p>Không mở đăng ký tự do để tránh lạm dụng điểm và nhầm quyền truy cập.</p>
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
      </section>
    </main>
  );
}
