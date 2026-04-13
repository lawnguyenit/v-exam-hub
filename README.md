# ExamHub

Giao diện đầu tiên cho website kiểm tra trực tuyến dùng Go làm server.

## Chạy bằng Go

```powershell
go run .
```

Mở `http://localhost:8080`.

## Cấu trúc trang

- `/`: trang index chỉ có hai lựa chọn vai trò.
- `/login/student`: form đăng nhập sinh viên / học sinh.
- `/login/teacher`: form đăng nhập giáo viên.
- `/student`: dashboard mẫu cho sinh viên / học sinh.
- `/student/exam`: trang làm bài độc lập.
- `/student/review`: trang xem lại bài thi độc lập.
- `/teacher`: dashboard mẫu cho giáo viên, đang để placeholder để làm sau.

Mỗi trang có file CSS và JS riêng:

- `static/css/login.css` và `static/js/login.js`
- `static/css/student.css` và `static/js/student.js`
- `static/css/exam.css` và `static/js/exam.js`
- `static/css/review.css` và `static/js/review.js`
- `static/css/teacher.css` và `static/js/teacher.js`

`static/css/common.css` chứa phần giao diện dùng chung.

## Dữ liệu

Hiện tại dữ liệu sinh viên được trả qua mock API trong Go:

- `/api/student/dashboard`
- `/api/student/exams/{id}`
- `/api/student/reviews/{id}`

Mock data đang nằm trong `internal/studentdata`. Khi thêm database, thay phần đọc dữ liệu ở module này bằng repository/service đọc từ DB, còn template và JS chỉ cần tiếp tục gọi API.

## Quy tắc đăng nhập hiện tại

Trang index không có nút đăng ký tự do. Bản UI test hiện nhận tài khoản/mật khẩu bất kỳ để đi qua flow, lưu session bằng `sessionStorage`.

Dashboard sinh viên lưu tiến trình làm bài bằng `localStorage` theo tài khoản và ca thi demo. Nếu người dùng thoát ra rồi đăng nhập lại cùng tài khoản trên cùng trình duyệt, đáp án và câu đang làm sẽ được mở lại, còn thời gian vẫn tính theo mốc `endAt` ban đầu.

Sau này backend sẽ xác thực tài khoản hoặc mã truy cập do trường cấp, đồng thời lưu bài làm và thời gian trên server để tránh sửa dữ liệu phía client.
