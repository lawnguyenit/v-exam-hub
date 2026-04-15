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
- `/student`: dashboard mẫu cho sinh viên / học sinh.
- `/student/exam`: trang làm bài độc lập.
- `/student/review`: trang xem lại bài thi độc lập.
- `/teacher`: dashboard mẫu cho giáo viên, đang để placeholder để làm sau.
- `/teacher/create`: trang tạo bài kiểm tra mới, có luồng upload file và AI format ở mức UI mock.

Frontend chính nằm trong `frontend/src`. Các trang cũ trong `templates/` đã được thay bằng React routes.

## Cấu trúc frontend

- `frontend/src/App.tsx`: chỉ giữ router cấp cao.
- `frontend/src/pages`: các màn theo domain như `auth`, `student`, `teacher`.
- `frontend/src/features`: phần nghiệp vụ có state riêng như làm bài, xem lại, thống kê giáo viên.
- `frontend/src/shared`: component dùng chung như brand, page shell.
- `frontend/src/lib`: helper dùng chung như auth guard và format.
- `frontend/src/styles`: CSS tách theo nhóm màn, `styles.css` chỉ import các file này.

## Dữ liệu

Hiện tại dữ liệu sinh viên được trả qua mock API trong Go:

- `/api/student/dashboard`
- `/api/student/exams/{id}`
- `/api/student/reviews/{id}`
- `/api/teacher/dashboard`
- `/api/teacher/exams/{id}`

Mock data đang nằm trong `internal/studentdata` và `internal/teacherdata`. Khi thêm database, thay phần đọc dữ liệu ở các module này bằng repository/service đọc từ DB, còn React frontend tiếp tục gọi API.

## Quy tắc đăng nhập hiện tại

Trang index không có nút đăng ký tự do. Bản UI test hiện nhận tài khoản/mật khẩu bất kỳ để đi qua flow, lưu session bằng `sessionStorage`.

Dashboard sinh viên lưu tiến trình làm bài bằng `localStorage` theo tài khoản và ca thi demo. Nếu người dùng thoát ra rồi đăng nhập lại cùng tài khoản trên cùng trình duyệt, đáp án và câu đang làm sẽ được mở lại, còn thời gian vẫn tính theo mốc `endAt` ban đầu.

Sau này backend sẽ xác thực tài khoản hoặc mã truy cập do trường cấp, đồng thời lưu bài làm và thời gian trên server để tránh sửa dữ liệu phía client.
