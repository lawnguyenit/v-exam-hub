# Frontend Data Mapping

This document maps the current React screens and Go mock APIs to the database schema. Use it as the migration checklist when replacing mock data with PostgreSQL repositories.

---

## 1. Seed order before question bank work

1. `roles`: seed `student`, `teacher`, `admin`.
2. `users`, `user_roles`: create controlled accounts only. Do not add public registration.
3. `student_profiles`, `teacher_profiles`: profile data for dashboard headers.
4. `classes`, `class_members`, `teacher_class_assignments`: class membership and teacher access.
5. `subjects`, `topics`, `question_tags`: filters needed by the question bank UI.
6. `import_batches`, `import_items`: staging area for Word/PDF/TXT parsing.
7. `question_bank`, `question_bank_options`, `question_bank_tags`, `question_attachments`: approved questions.
8. `exams`, `exam_questions`, `exam_targets`: teacher-created exam configuration.
9. `exam_versions`, `exam_version_questions`, `exam_version_question_options`: published snapshots.
10. `exam_attempts`, `attempt_questions`, `attempt_question_options`, `student_answers`, `student_answer_options`, `attempt_events`: runtime and review data.

---

## 2. Current API to table mapping

### `/api/student/dashboard`

Current frontend fields:
- `profile`: `users`, `student_profiles`, `class_members`, `classes`
- `summary.availableCount`: open exams targeted to the student's active class where `exam_status = 'open'`
- `summary.plannedCount`: scheduled exams targeted to the student's active class where `exam_status = 'scheduled'`
- `summary.latestScore`: latest submitted `exam_attempts.score_final`
- `availableExams`: `exams`, `exam_targets`, `classes`, latest matching `exam_attempts`
- `plannedExams`: `exams`, `exam_targets`, `classes`
- `history`: submitted or expired `exam_attempts` joined with `exams`

### `/api/student/exams/{id}`

Use this flow:
1. Find the targeted `exams` row and latest published `exam_versions` row.
2. If the student has an active `exam_attempts` row, reuse it.
3. If not, create `exam_attempts` with fixed `started_at`, `end_at`, `duration_seconds_snapshot`, and generated attempt snapshots.
4. Read questions from `attempt_questions` and options from `attempt_question_options`, not directly from `question_bank`.
5. Save progress into `student_answers`, `student_answer_options`, `exam_attempts.current_question_order`, and `exam_attempts.last_saved_at`.

Timer rule: the browser can display the timer, but `exam_attempts.end_at` is the source of truth.

### `/api/student/reviews/{id}`

Use `exam_attempts`, `attempt_questions`, `attempt_question_options`, `student_answers`, and `student_answer_options`.

Review must show:
- selected answer
- correct answer snapshot
- question explanation snapshot
- score awarded

Do not read correctness from live `question_bank` here, because the teacher may edit a bank question after the student submits.

### `/api/teacher/dashboard`

Use:
- `teacher_profiles` for profile block
- `teacher_class_assignments` and `classes` to scope visible classes
- `exams.created_by_user_id` plus `exam_targets.class_id` to list exams the teacher can manage
- aggregate submitted/total counts from `exam_targets`, `class_members`, and `exam_attempts`

Important UI mapping:
- `examType` comes from `exams.exam_mode`
- `status` comes from `exams.exam_status`

Do not merge `practice/official` into `open/scheduled/closed`.

### `/api/teacher/exams/{id}`

Use:
- `exams`, `exam_targets`, `classes` for header and metrics
- `exam_questions` or `exam_versions` for question count
- `exam_attempts` for submitted count, average, best score, and average duration
- `student_answers` for score distribution
- `attempt_questions` and `student_answers` for difficult-question statistics
- `attempt_events` and `exam_attempts.client_last_seen_at` for live room status

For modal "Xem" details per student, read from the selected student's `exam_attempts` snapshot tables.

---

## 3. First repository split

Keep repository modules close to the UI/API boundary:

- `internal/authrepo`: login, role check, account status.
- `internal/studentrepo`: student dashboard, exam workspace, review.
- `internal/teacherrepo`: teacher dashboard and exam statistics.
- `internal/questionrepo`: import review, bank filters, question CRUD.
- `internal/attemptrepo`: start/resume attempt, save answer, submit attempt, append events.

The handler layer should only translate HTTP request/response shapes. It should not contain SQL joins.

---

## 4. Minimum query views to build first

Build these query results before making the full question-bank editor:

1. Student dashboard list: exams visible to one student.
2. Teacher exam list: latest 3 or filtered exams visible to one teacher.
3. Teacher exam detail: one selected exam with stable metrics and scrollable student table.
4. Student attempt resume: one attempt with remaining time and current question order.
5. Student review detail: one submitted attempt with selected/correct answers.

After these work from DB, the question bank can be added without breaking the current UI.
