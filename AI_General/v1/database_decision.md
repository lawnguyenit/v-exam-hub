# Database Decision

## Quyết định

Dùng PostgreSQL và lấy schema DBML hiện tại làm nguồn thiết kế chính cho hệ thống thi.

File tham chiếu:

- `AI_General/v1/web_exam_schema (1).dbml`

Schema này phù hợp với hướng sản phẩm hiện tại vì đã tách đúng các domain lớn:

- Identity & access: `roles`, `users`, `user_roles`, `student_profiles`, `teacher_profiles`
- Classroom: `classes`, `class_members`
- Import / staging: `import_batches`, `import_items`
- Question bank: `question_bank`, `question_bank_options`
- Exam config: `exams`, `exam_questions`, `exam_targets`
- Exam version history: `exam_versions`, `exam_version_questions`, `exam_version_question_options`
- Runtime attempts: `exam_attempts`, `attempt_questions`, `attempt_question_options`, `student_answers`, `student_answer_options`, `attempt_events`
- Audit & monetization: `audit_logs`, `ads`, `ad_impressions`

## Lý do giữ cấu trúc version/snapshot

Khi giáo viên chỉnh câu hỏi hoặc đáp án sau khi bài đã mở, bài làm cũ của sinh viên vẫn phải giữ đúng nội dung tại thời điểm làm bài. Vì vậy cần các bảng snapshot:

- `exam_versions`
- `exam_version_questions`
- `exam_version_question_options`
- `attempt_questions`
- `attempt_question_options`

Trang xem lại bài thi sẽ đọc từ dữ liệu snapshot này để hiển thị đáp án đúng/sai, không đọc trực tiếp từ question bank hiện tại.

## Mapping với module Go hiện tại

- `internal/studentdata`: mock tạm cho dashboard, làm bài, xem lại bài thi. Sau này thay bằng repository đọc `exam_attempts`, `attempt_questions`, `student_answers`.
- `internal/teacherdata`: mock tạm cho dashboard giáo viên và thống kê. Sau này thay bằng query từ `exams`, `exam_targets`, `exam_attempts`, `student_answers`, `attempt_events`.
- Module tạo bài mới của giáo viên sẽ đi qua `import_batches` và `import_items` trước khi ghi vào `question_bank`.

## Hướng triển khai database tiếp theo

1. Chuyển DBML sang migration PostgreSQL.
2. Seed các role cơ bản: `student`, `teacher`, `admin`.
3. Tạo repository theo domain, không query DB trực tiếp trong handler.
4. Thay API mock hiện tại bằng repository:
   - `/api/student/dashboard`
   - `/api/student/exams/{id}`
   - `/api/student/reviews/{id}`
   - `/api/teacher/dashboard`
   - `/api/teacher/exams/{id}`
5. Khi làm bài chính thức, server là nguồn thời gian duy nhất qua `exam_attempts.end_at`. Client không được tự quyết định timer.
