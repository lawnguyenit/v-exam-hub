# Domain: Question Bank and Exam Assembly

This document explains how raw teaching material becomes durable question assets and then becomes an exam.

---

## 1. Purpose

The question bank is the durable knowledge layer of the system.

It must support:
- reuse across many exams
- import from inconsistent teacher files
- filtering by topic, difficulty, or status later
- safe versioning when exams are already published

---

## 2. Data path

```mermaid
flowchart TD
    A[Teacher file / pasted text] --> B[import_batches]
    B --> C[import_items]
    C --> D[Manual review / normalization]
    D --> E[question_bank]
    E --> F[question_bank_options]
    E --> G[exam_questions]
    G --> H[exam_versions]
```

---

## 3. Why questions do not belong directly to exams

If a question belongs only to one exam, the system becomes exam-centric and future growth becomes expensive.

By keeping `question_bank` as the source layer:
- the same question can appear in many exams
- import is separated from delivery
- stats can later be attached to bank items
- a teacher can build variants without duplicating raw text too early

---

## 3.1 Classification layer

Before the UI can show a usable question bank, questions need stable filters:

- `subjects`: môn học hoặc nhóm kiến thức lớn.
- `topics`: chủ đề con, có thể lồng nhau bằng `parent_topic_id`.
- `question_tags`: nhãn linh hoạt như `go`, `channel`, `tcp`, `easy-to-misread`.
- `question_bank_tags`: many-to-many giữa câu hỏi và tag.
- `question_attachments`: hình ảnh/tài liệu đi kèm câu hỏi.

This keeps the bank searchable when it grows beyond a few teacher-uploaded files.

---

## 4. Assembly rule

```mermaid
flowchart LR
    QB[question_bank] --> EQ[exam_questions]
    E[exams] --> EQ
    EQ --> EV[exam_versions]
```

Interpretation:
- `question_bank` holds source questions.
- `question_bank.default_points` holds the suggested default score.
- `exam_questions` says which bank questions are included in an exam.
- `exam_questions.points_override` changes score for a specific exam without mutating the bank question.
- `exam_versions` freezes a publishable snapshot.

---

## 5. Design warning

Do not overload `question_bank` with runtime fields such as:
- current student answer count
- current attempt order
- active exam timer state

Those belong elsewhere.

---

## 6. Expected future extensions

- tags / topics / subjects
- difficulty levels
- media attachments
- imported document confidence scores
- AI-assisted parse suggestions
- item statistics such as discrimination or difficulty index
