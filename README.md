# ExamHub

Website kiểm tra trực tuyến dùng React cho frontend và Go cho backend API.

## Chạy bản production local

```powershell
cd frontend
npm install
npm run build
cd ..
go run .
```

Mở `http://localhost:8080`.

Go sẽ serve file build trong `frontend/dist` và giữ các API `/api/...`.

## Chạy khi phát triển frontend

Chạy Go API:

```powershell
go run .
```

Chạy Vite dev server:

```powershell
cd frontend
npm run dev
```

Mở URL Vite in ra terminal. Vite proxy `/api` sang `http://localhost:8080`.

## Cấu trúc trang

- `/`: trang index chỉ có hai lựa chọn vai trò.
- `/login/student`: form đăng nhập sinh viên / học sinh.
- `/login/teacher`: form đăng nhập giáo viên.
- `/student`: dashboard sinh viên / học sinh đọc từ database.
- `/student/exam`: trang làm bài độc lập.
- `/student/review`: trang xem lại bài thi độc lập.
- `/teacher`: dashboard giáo viên đọc danh sách bài, thống kê và sinh viên từ database.
- `/teacher/create`: trang tạo bài kiểm tra mới, upload file và parser local trước khi lưu ngân hàng câu hỏi.

Frontend chính nằm trong `frontend/src`. Các trang cũ trong `templates/` đã được thay bằng React routes.

## Cấu trúc frontend

- `frontend/src/App.tsx`: chỉ giữ router cấp cao.
- `frontend/src/pages`: các màn theo domain như `auth`, `student`, `teacher`.
- `frontend/src/features`: phần nghiệp vụ có state riêng như làm bài, xem lại, thống kê giáo viên.
- `frontend/src/shared`: component dùng chung như brand, page shell.
- `frontend/src/lib`: helper dùng chung như auth guard và format.
- `frontend/src/styles`: CSS tách theo nhóm màn, `styles.css` chỉ import các file này.

## Dữ liệu

Các màn chính gọi Go API và đọc/ghi PostgreSQL:

- `/api/student/dashboard`
- `/api/student/exams/{id}`
- `/api/student/reviews/{id}`
- `/api/teacher/dashboard`
- `/api/teacher/exams/{id}`

Các module `internal/studentdata`, `internal/teacherdata`, `internal/importdata` và `internal/accountdata` là lớp truy cập dữ liệu hiện tại. React frontend không giữ mock nghiệp vụ; nó chỉ gọi API.

## Quy tắc đăng nhập hiện tại

Trang index không có nút đăng ký tự do. Đăng nhập dùng tài khoản được cấp trong database, lưu phiên frontend bằng `sessionStorage`.

Tiến trình làm bài được lưu trong `exam_attempts`, `attempt_questions`, `student_answers` và `student_answer_options`. Nếu người dùng thoát ra rồi đăng nhập lại, backend khôi phục lần làm còn hạn; thời gian vẫn tính theo `end_at` trong database.
