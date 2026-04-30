# Project structure rules

These rules are the default placement rules for future work. If a new file does not fit one of these folders, create the correct domain folder first instead of placing it in the project root.

## Backend

Root package is only for the executable shell:

- `main.go`
- `server.go`
- temporary root route files until they are moved into `internal/httpapi`
- database startup glue until migrations are extracted

Do not add generic helpers to root.

Use these packages:

- `internal/config`: runtime/env configuration only
- `internal/httpapi`: HTTP middleware, response helpers, static serving, route-level transport helpers
- `internal/authsession`: login session, auth guard, ownership guard
- `internal/storage`: local/S3/EFS asset storage boundary
- `internal/importdata`: import parsing, import item persistence, question-bank import workflow
- `internal/accountdata`: user, teacher, student account persistence
- `internal/studentdata`: student dashboard/exam/attempt persistence
- `internal/teacherdata`: teacher dashboard/exam/class/question-bank persistence

Future target:

```text
cmd/server/main.go
internal/httpapi/router.go
internal/httpapi/handlers/auth.go
internal/httpapi/handlers/student.go
internal/httpapi/handlers/teacher.go
internal/httpapi/handlers/admin.go
internal/db/migrations
internal/importdata/adapters/*
internal/storage/local
internal/storage/s3
```

## Frontend

Root `frontend/src` should hold only app composition:

- `main.tsx`
- `App.tsx`
- top-level stylesheet import files

Domain files go into:

- `src/pages`: route-level pages
- `src/features`: feature/business components with state
- `src/shared`: reusable UI components
- `src/lib`: browser/client helpers such as auth storage and formatting
- `src/styles`: CSS grouped by screen or feature

Do not place `utils.ts`, `config.ts`, or `storage.ts` directly under `frontend/src`.

## Import adapters

Every importer should be explicit and testable:

- one adapter per format or standard
- golden fixtures for each supported standard
- no parser rule hidden in UI code
- images/assets must flow through `internal/storage`

Examples:

- Moodle XML
- GIFT
- Aiken
- plain TXT/CSV table
- DOCX
- legacy DOC through LibreOffice
- PDF text extraction
- PDF scan/OCR marked as demo until reliable
