# ExamHub

Giao diện đầu tiên cho website kiểm tra trực tuyến dùng Go làm server.

## Chạy bằng Go

```powershell
go run .
```

Mở `http://localhost:8080`.

## Cấu trúc trang

- `/`: trang index đăng nhập, chỉ có hai lựa chọn vai trò.
- `/student`: dashboard mẫu cho sinh viên / học sinh.
- `/teacher`: dashboard mẫu cho giáo viên, đang để placeholder để làm sau.

Mỗi trang có file CSS và JS riêng:

- `static/css/login.css` và `static/js/login.js`
- `static/css/student.css` và `static/js/student.js`
- `static/css/teacher.css` và `static/js/teacher.js`

`static/css/common.css` chứa phần giao diện dùng chung.

## Quy tắc đăng nhập hiện tại

Trang index không có nút đăng ký tự do. Sau này backend sẽ chỉ xác thực tài khoản hoặc mã truy cập do trường cấp để tránh lạm dụng điểm và nhầm quyền truy cập.
