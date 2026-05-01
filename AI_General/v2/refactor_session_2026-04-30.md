# Refactor session 2026-04-30

## Current goal

Make the project maintainable enough for production growth:

- adapter-based import handling for TXT/CSV/Aiken/GIFT/Moodle XML/DOCX/DOC/PDF
- durable image/asset storage for questions and options
- adding questions into existing question sources
- production-grade server sessions
- AWS/ECS/RDS deployment without local-path assumptions

## Changes made in this session

### Backend entrypoint split

The old root `main.go` mixed startup, routing, handlers, database compatibility, and HTTP helpers.

It has been split into:

- `main.go`: minimal entrypoint only
- `server.go`: server startup and route registration
- `database.go`: DB connection, schema presence checks, compatibility patches
- `handlers_auth.go`: auth, session bootstrap login/logout/me, admin teacher creation, login limiter
- `handlers_student.go`: student attempt endpoints
- `handlers_teacher.go`: teacher profile, dashboard detail, question import, class/student management endpoints
- `internal/httpapi`: JSON response helpers, static frontend serving, runtime CORS wrapper
- `internal/authsession`: DB-backed session/cookie guard helpers

This is still package `main` intentionally. It is the safe intermediate step before moving route groups into `internal/httpapi`.

### Import extraction boundary

Added `internal/importdata/extractors.go`.

`parser.go` no longer owns the first file-kind switch directly. It now calls `contentExtractors()[kind]`.

Current extractor functions:

- `extractTextFile`
- `extractCSVFile`
- `extractMoodleXMLFile`
- `extractDocxFile`
- `extractLegacyDocFile`
- `extractPDFFile`

This creates the first adapter seam: each file format can now be moved into its own adapter without changing `ParseUpload`.

### Vietnamese answer detection fix

The parser previously matched some answer lines only in mojibake form. It now recognizes both real Vietnamese and legacy mojibake variants:

- `Đáp án: A`
- `đ/a`
- `D/A`
- `Answer: A`
- existing mojibake variants

This fixed a backend test failure where `Đáp án: A` was being appended into option B.

### CORS credential support

`cors.go` now sets `Access-Control-Allow-Credentials: true` when the request origin matches an explicit configured origin.

This matters for production/session mode where auth uses the `examhub_session` HttpOnly cookie. Wildcard CORS is still accepted for local/dev-style access, but credentialed browser requests need a concrete origin.

### Runtime config boundary

Added `internal/config`.

Runtime settings are now loaded once through `config.Load()`:

- `APP_ENV`
- `PORT`
- `DB_URL`
- `DB_STARTUP_TIMEOUT`
- `CORS_ALLOWED_ORIGINS`

`server.go`, `database.go`, and `cors.go` now receive runtime config instead of reading those env vars directly.

### Local storage boundary

Added `internal/storage`.

Import source files and extracted assets still use local disk by default, but they now go through one boundary:

- default root: `data/imports`
- override: `IMPORT_STORAGE_DIR`

This prepares the project for S3-backed import/question assets without rewriting parser and handler code again.

### Structure rules

Added `AI_General/v2/project_structure_rules.md` as the placement rule for future files.

Also moved frontend auth storage from `frontend/src/storage.ts` to `frontend/src/lib/authStorage.ts` so root `src` stays reserved for app composition.

Moved frontend API exports from `frontend/src/api.ts` to domain modules under `frontend/src/api`:

- `types.ts`
- `client.ts`
- `authApi.ts`
- `studentApi.ts`
- `teacherApi.ts`
- `importApi.ts`
- `adminApi.ts`
- `index.ts`

The old `api.ts` is now only a compatibility facade.

Added `scripts/check_structure.ps1` to catch future helper/config/session/storage files placed back into root folders.

## Verification

Passed:

```powershell
go test ./...
```

Frontend build:

```powershell
cd D:\v-exam-hub\frontend
npm run build
```

Passed from the real path `D:\v-exam-hub`.

Re-ran backend tests and frontend build after the runtime config/storage extraction; both passed.

Known environment issue:

`D:\website-exam` is a Windows junction to `D:\v-exam-hub`. Vite/Rolldown fails when building through the junction path because it resolves emitted HTML as `../../v-exam-hub/frontend/index.html`.

Use the real path for frontend builds:

```powershell
cd D:\v-exam-hub\frontend
npm run build
```

## Next refactor targets

### 1. Move route groups into internal HTTP packages

Target:

```text
internal/httpapi/router.go
internal/httpapi/middleware/auth.go
internal/httpapi/handlers/auth.go
internal/httpapi/handlers/student.go
internal/httpapi/handlers/teacher.go
internal/httpapi/handlers/admin.go
```

Root package should keep only `cmd/server/main.go`.

### 2. Move DB compatibility into migrations

Current runtime still has:

- `ensureCoreSchema`
- `ensureDatabaseCompatibility`
- `ensureSessionSchema`

Production target:

```text
db/migrations
db/seeds/local
db/seeds/production-bootstrap
```

Backend should check migration version, not mutate schema freely during request server startup.

### 3. Product session model

Current session is DB-backed and cookie-based, but still needs:

- session config from env: secure cookie, same-site policy, TTL
- `/api/auth/me` as the only frontend source of truth
- frontend route guards should not trust localStorage
- optional active-session policy: single device or multiple devices
- CSRF strategy if frontend and backend share cookie auth across domains

### 4. Import adapter package layout

Move adapter logic toward:

```text
internal/importdata/pipeline.go
internal/importdata/adapters/txt
internal/importdata/adapters/csv
internal/importdata/adapters/aiken
internal/importdata/adapters/gift
internal/importdata/adapters/moodlexml
internal/importdata/adapters/docx
internal/importdata/adapters/pdf
```

Each adapter should have golden fixture tests under `input_test` or `internal/importdata/testdata`.

### 5. Asset storage abstraction

Current extracted assets are stored locally under `data/imports`.

Production target:

```text
internal/storage
internal/storage/local
internal/storage/s3
```

Question and option rendering should reference asset IDs/URLs instead of relying only on `[Hình N]`.

### 6. Question source append/update

Need a durable workflow for:

- save import as a new source
- append only new questions to an existing source
- update existing matched questions
- keep rejected/deleted import items out of the active bank
- maintain fingerprint-based duplicate detection

### 7. Frontend cleanup

Split `frontend/src/api.ts` into domain clients:

```text
frontend/src/api/authApi.ts
frontend/src/api/studentApi.ts
frontend/src/api/teacherApi.ts
frontend/src/api/importApi.ts
frontend/src/api/adminApi.ts
```

Then split large pages into feature components and hooks.
