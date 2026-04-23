BEGIN;

-- Local-only recovery seed.
-- Do not mount this file in production. Keep it for local restore after
-- `docker compose down -v` so login and dashboard flows are usable again.

INSERT INTO users (username, password_hash, account_status)
VALUES
    ('admin', '123456', 'active'),
    ('gv-cntt-01', '123456', 'active'),
    ('lnit', '123456', 'active')
ON CONFLICT (username) DO UPDATE
SET password_hash = EXCLUDED.password_hash,
    account_status = EXCLUDED.account_status,
    updated_at = NOW();

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code = 'admin'
WHERE u.username = 'admin'
ON CONFLICT DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code = 'teacher'
WHERE u.username = 'gv-cntt-01'
ON CONFLICT DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code = 'student'
WHERE u.username = 'lnit'
ON CONFLICT DO NOTHING;

INSERT INTO teacher_profiles (user_id, teacher_code, full_name, email, phone, department, profile_status)
SELECT
    u.id,
    'GV-CNTT-01',
    'Giáo viên CNTT',
    'gv-cntt-01@local.examhub',
    '0900000001',
    'Khoa Công nghệ thông tin',
    'active'
FROM users u
WHERE u.username = 'gv-cntt-01'
ON CONFLICT (user_id) DO UPDATE
SET teacher_code = EXCLUDED.teacher_code,
    full_name = EXCLUDED.full_name,
    email = EXCLUDED.email,
    phone = EXCLUDED.phone,
    department = EXCLUDED.department,
    profile_status = EXCLUDED.profile_status,
    updated_at = NOW();

INSERT INTO student_profiles (user_id, student_code, full_name, email, phone, profile_status)
SELECT
    u.id,
    '22004320',
    'Nguyễn Lâm Nguyên',
    'lawnguyenit@gmail.com',
    '0900000002',
    'active'
FROM users u
WHERE u.username = 'lnit'
ON CONFLICT (user_id) DO UPDATE
SET student_code = EXCLUDED.student_code,
    full_name = EXCLUDED.full_name,
    email = EXCLUDED.email,
    phone = EXCLUDED.phone,
    profile_status = EXCLUDED.profile_status,
    updated_at = NOW();

INSERT INTO classes (class_code, class_name, school_year, major, homeroom_teacher_user_id, created_by_user_id, class_status)
SELECT
    'CNTT K48',
    'Công nghệ thông tin K48',
    '2025-2026',
    'Công nghệ thông tin',
    teacher_user.id,
    admin_user.id,
    'active'
FROM users admin_user
JOIN users teacher_user ON teacher_user.username = 'gv-cntt-01'
WHERE admin_user.username = 'admin'
ON CONFLICT (class_code) DO UPDATE
SET class_name = EXCLUDED.class_name,
    school_year = EXCLUDED.school_year,
    major = EXCLUDED.major,
    homeroom_teacher_user_id = EXCLUDED.homeroom_teacher_user_id,
    created_by_user_id = EXCLUDED.created_by_user_id,
    class_status = EXCLUDED.class_status,
    updated_at = NOW();

INSERT INTO teacher_class_assignments (class_id, teacher_user_id, permission, assignment_status, assigned_by_user_id)
SELECT
    c.id,
    teacher_user.id,
    'editor',
    'active',
    admin_user.id
FROM classes c
JOIN users teacher_user ON teacher_user.username = 'gv-cntt-01'
JOIN users admin_user ON admin_user.username = 'admin'
WHERE c.class_code = 'CNTT K48'
ON CONFLICT (class_id, teacher_user_id) DO UPDATE
SET permission = EXCLUDED.permission,
    assignment_status = EXCLUDED.assignment_status,
    assigned_by_user_id = EXCLUDED.assigned_by_user_id,
    updated_at = NOW();

INSERT INTO class_members (class_id, student_user_id, member_status)
SELECT
    c.id,
    student_user.id,
    'active'
FROM classes c
JOIN users student_user ON student_user.username = 'lnit'
WHERE c.class_code = 'CNTT K48'
ON CONFLICT (class_id, student_user_id) DO UPDATE
SET member_status = EXCLUDED.member_status,
    updated_at = NOW();

COMMIT;
