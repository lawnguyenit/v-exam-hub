-- RDS production bootstrap schema for v-exam-hub (PostgreSQL)
-- Prepared for AWS RDS bootstrap on 2026-04-25.
--
-- Purpose:
-- 1) Use this file to initialize a fresh AWS RDS PostgreSQL database.
-- 2) Keep production data minimal: schema, roles, and the first admin account.
-- 3) Do not include local/demo exams, students, teachers, or question banks.
--
-- Notes:
-- - Run this once against an empty RDS database.
-- - If the database already has objects, this file can fail because base enum
--   and table statements are intentionally strict.
-- - After first login, change the bootstrap admin password immediately.
--
-- Design goals:
-- 1) Separate core assets, delivery config, runtime data, and history/audit.
-- 2) Preserve historical correctness with exam_versions and attempt snapshots.
-- 3) Keep future changes additive where possible.

BEGIN;

-- =========================================================
-- Utility
-- =========================================================

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- =========================================================
-- Enums
-- =========================================================

CREATE TYPE account_status_enum AS ENUM ('active', 'locked', 'disabled');
CREATE TYPE profile_status_enum AS ENUM ('active', 'inactive');
CREATE TYPE class_status_enum AS ENUM ('active', 'archived');
CREATE TYPE class_member_status_enum AS ENUM ('active', 'removed', 'suspended');
CREATE TYPE question_type_enum AS ENUM ('single_choice', 'multiple_choice', 'text');
CREATE TYPE question_status_enum AS ENUM ('draft', 'active', 'archived');
CREATE TYPE import_source_type_enum AS ENUM ('doc', 'docx', 'pdf', 'txt', 'pasted_text', 'xlsx', 'csv', 'other');
CREATE TYPE import_parse_status_enum AS ENUM ('pending', 'parsed', 'needs_ocr', 'needs_conversion', 'failed', 'reviewed');
CREATE TYPE import_review_status_enum AS ENUM ('pending', 'approved', 'rejected');
CREATE TYPE exam_mode_enum AS ENUM ('practice', 'official', 'attendance');
CREATE TYPE exam_status_enum AS ENUM ('draft', 'scheduled', 'open', 'closed', 'archived');
CREATE TYPE attempt_status_enum AS ENUM ('in_progress', 'submitted', 'expired', 'cancelled');
CREATE TYPE grading_status_enum AS ENUM ('auto_graded', 'manual_pending', 'finalized');
CREATE TYPE submit_source_enum AS ENUM ('manual', 'auto_timeout', 'admin_force_submit');
CREATE TYPE attempt_event_type_enum AS ENUM ('login', 'start', 'answer_saved', 'tab_hidden', 'tab_visible', 'reconnect', 'submit', 'timeout', 'heartbeat');
CREATE TYPE ad_placement_enum AS ENUM ('dashboard_side', 'after_submit');
CREATE TYPE generic_status_enum AS ENUM ('active', 'inactive', 'archived');

-- =========================================================
-- Identity & access
-- =========================================================

CREATE TABLE roles (
    id SMALLSERIAL PRIMARY KEY,
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    account_status account_status_enum NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_roles (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id SMALLINT NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE student_profiles (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    student_code VARCHAR(50) NOT NULL UNIQUE,
    full_name VARCHAR(255) NOT NULL,
    avatar_url TEXT,
    email VARCHAR(255),
    phone VARCHAR(30),
    profile_status profile_status_enum NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE teacher_profiles (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    teacher_code VARCHAR(50) NOT NULL UNIQUE,
    full_name VARCHAR(255) NOT NULL,
    avatar_url TEXT,
    email VARCHAR(255),
    phone VARCHAR(30),
    department VARCHAR(255),
    profile_status profile_status_enum NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =========================================================
-- Classroom
-- =========================================================

CREATE TABLE classes (
    id BIGSERIAL PRIMARY KEY,
    class_code VARCHAR(50) NOT NULL UNIQUE,
    class_name VARCHAR(255) NOT NULL,
    school_year VARCHAR(50),
    major VARCHAR(255),
    homeroom_teacher_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    class_status class_status_enum NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE class_members (
    id BIGSERIAL PRIMARY KEY,
    class_id BIGINT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    student_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    member_status class_member_status_enum NOT NULL DEFAULT 'active',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    left_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_class_members UNIQUE (class_id, student_user_id),
    CONSTRAINT ck_class_member_dates CHECK (left_at IS NULL OR left_at >= joined_at)
);

-- =========================================================
-- Import / staging for messy teacher source files
-- =========================================================

CREATE TABLE import_batches (
    id BIGSERIAL PRIMARY KEY,
    uploaded_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    source_name VARCHAR(255),
    source_type import_source_type_enum NOT NULL DEFAULT 'other',
    file_sha256 CHAR(64),
    content_fingerprint CHAR(64),
    raw_content TEXT,
    parse_status import_parse_status_enum NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE import_items (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES import_batches(id) ON DELETE CASCADE,
    item_order INT NOT NULL,
    source_order INT,
    raw_question_text TEXT NOT NULL,
    parsed_question_json JSONB,
    review_status import_review_status_enum NOT NULL DEFAULT 'pending',
    review_note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_import_item_order UNIQUE (batch_id, item_order)
);

-- =========================================================
-- Question bank (asset layer)
-- =========================================================

CREATE TABLE question_bank (
    id BIGSERIAL PRIMARY KEY,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    import_item_id BIGINT REFERENCES import_items(id) ON DELETE SET NULL,
    question_type question_type_enum NOT NULL,
    content TEXT NOT NULL,
    explanation TEXT,
    difficulty SMALLINT,
    question_status question_status_enum NOT NULL DEFAULT 'draft',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_question_difficulty CHECK (difficulty IS NULL OR difficulty BETWEEN 1 AND 5)
);

CREATE TABLE question_bank_options (
    id BIGSERIAL PRIMARY KEY,
    question_id BIGINT NOT NULL REFERENCES question_bank(id) ON DELETE CASCADE,
    option_order INT NOT NULL,
    content TEXT NOT NULL,
    is_correct BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_question_option_order UNIQUE (question_id, option_order)
);

-- =========================================================
-- Exams (delivery config layer)
-- =========================================================

CREATE TABLE exams (
    id BIGSERIAL PRIMARY KEY,
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    exam_mode exam_mode_enum NOT NULL DEFAULT 'practice',
    duration_seconds INT NOT NULL,
    total_points NUMERIC(10,2) NOT NULL DEFAULT 0,
    exam_status exam_status_enum NOT NULL DEFAULT 'draft',
    max_attempts_per_student INT NOT NULL DEFAULT 1,
    shuffle_questions BOOLEAN NOT NULL DEFAULT FALSE,
    shuffle_options BOOLEAN NOT NULL DEFAULT FALSE,
    show_result_immediately BOOLEAN NOT NULL DEFAULT FALSE,
    allow_review BOOLEAN NOT NULL DEFAULT FALSE,
    access_code VARCHAR(100),
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_exam_duration_positive CHECK (duration_seconds > 0),
    CONSTRAINT ck_exam_points_non_negative CHECK (total_points >= 0),
    CONSTRAINT ck_exam_attempts_non_negative CHECK (max_attempts_per_student >= 0),
    CONSTRAINT ck_exam_time_window CHECK (start_time IS NULL OR end_time IS NULL OR end_time > start_time)
);

CREATE TABLE exam_questions (
    id BIGSERIAL PRIMARY KEY,
    exam_id BIGINT NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    question_id BIGINT NOT NULL REFERENCES question_bank(id) ON DELETE RESTRICT,
    question_order INT NOT NULL,
    points_override NUMERIC(10,2),
    is_required BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_exam_question_order UNIQUE (exam_id, question_order),
    CONSTRAINT uq_exam_question UNIQUE (exam_id, question_id),
    CONSTRAINT ck_exam_question_points_non_negative CHECK (points_override IS NULL OR points_override >= 0)
);

CREATE TABLE exam_targets (
    id BIGSERIAL PRIMARY KEY,
    exam_id BIGINT NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    class_id BIGINT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_exam_target UNIQUE (exam_id, class_id)
);

-- =========================================================
-- Exam versions (historical freeze layer)
-- =========================================================

CREATE TABLE exam_versions (
    id BIGSERIAL PRIMARY KEY,
    exam_id BIGINT NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    version_no INT NOT NULL,
    title_snapshot VARCHAR(255) NOT NULL,
    description_snapshot TEXT,
    exam_mode_snapshot exam_mode_enum NOT NULL,
    duration_seconds_snapshot INT NOT NULL,
    total_points_snapshot NUMERIC(10,2) NOT NULL,
    exam_status_snapshot exam_status_enum NOT NULL,
    shuffle_questions_snapshot BOOLEAN NOT NULL DEFAULT FALSE,
    shuffle_options_snapshot BOOLEAN NOT NULL DEFAULT FALSE,
    show_result_immediately_snapshot BOOLEAN NOT NULL DEFAULT FALSE,
    allow_review_snapshot BOOLEAN NOT NULL DEFAULT FALSE,
    start_time_snapshot TIMESTAMPTZ,
    end_time_snapshot TIMESTAMPTZ,
    published_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_exam_version UNIQUE (exam_id, version_no),
    CONSTRAINT ck_exam_version_duration_positive CHECK (duration_seconds_snapshot > 0),
    CONSTRAINT ck_exam_version_points_non_negative CHECK (total_points_snapshot >= 0),
    CONSTRAINT ck_exam_version_time_window CHECK (
        start_time_snapshot IS NULL OR end_time_snapshot IS NULL OR end_time_snapshot > start_time_snapshot
    )
);

CREATE TABLE exam_version_questions (
    id BIGSERIAL PRIMARY KEY,
    exam_version_id BIGINT NOT NULL REFERENCES exam_versions(id) ON DELETE CASCADE,
    source_question_id BIGINT REFERENCES question_bank(id) ON DELETE SET NULL,
    question_order INT NOT NULL,
    question_type_snapshot question_type_enum NOT NULL,
    content_snapshot TEXT NOT NULL,
    explanation_snapshot TEXT,
    points_snapshot NUMERIC(10,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_exam_version_question_order UNIQUE (exam_version_id, question_order),
    CONSTRAINT ck_exam_version_question_points_non_negative CHECK (points_snapshot >= 0)
);

CREATE TABLE exam_version_question_options (
    id BIGSERIAL PRIMARY KEY,
    exam_version_question_id BIGINT NOT NULL REFERENCES exam_version_questions(id) ON DELETE CASCADE,
    option_order INT NOT NULL,
    content_snapshot TEXT NOT NULL,
    is_correct_snapshot BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_exam_version_option_order UNIQUE (exam_version_question_id, option_order)
);

-- =========================================================
-- Runtime / attempt layer
-- =========================================================

CREATE TABLE exam_attempts (
    id BIGSERIAL PRIMARY KEY,
    exam_id BIGINT NOT NULL REFERENCES exams(id) ON DELETE RESTRICT,
    exam_version_id BIGINT NOT NULL REFERENCES exam_versions(id) ON DELETE RESTRICT,
    student_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    attempt_no INT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_at TIMESTAMPTZ NOT NULL,
    submitted_at TIMESTAMPTZ,
    attempt_status attempt_status_enum NOT NULL DEFAULT 'in_progress',
    duration_seconds_snapshot INT NOT NULL,
    total_points_snapshot NUMERIC(10,2) NOT NULL DEFAULT 0,
    score_raw NUMERIC(10,2) NOT NULL DEFAULT 0,
    score_final NUMERIC(10,2) NOT NULL DEFAULT 0,
    grading_status grading_status_enum NOT NULL DEFAULT 'auto_graded',
    client_last_seen_at TIMESTAMPTZ,
    ip_address INET,
    user_agent TEXT,
    submit_source submit_source_enum,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_exam_attempt_no UNIQUE (exam_id, student_user_id, attempt_no),
    CONSTRAINT ck_attempt_no_positive CHECK (attempt_no > 0),
    CONSTRAINT ck_attempt_end_after_start CHECK (end_at > started_at),
    CONSTRAINT ck_attempt_duration_positive CHECK (duration_seconds_snapshot > 0),
    CONSTRAINT ck_attempt_points_non_negative CHECK (total_points_snapshot >= 0),
    CONSTRAINT ck_attempt_scores_non_negative CHECK (score_raw >= 0 AND score_final >= 0),
    CONSTRAINT ck_attempt_submit_time CHECK (submitted_at IS NULL OR submitted_at >= started_at)
);

CREATE TABLE attempt_questions (
    id BIGSERIAL PRIMARY KEY,
    attempt_id BIGINT NOT NULL REFERENCES exam_attempts(id) ON DELETE CASCADE,
    exam_version_question_id BIGINT REFERENCES exam_version_questions(id) ON DELETE SET NULL,
    question_order INT NOT NULL,
    question_type_snapshot question_type_enum NOT NULL,
    content_snapshot TEXT NOT NULL,
    explanation_snapshot TEXT,
    points_snapshot NUMERIC(10,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_attempt_question_order UNIQUE (attempt_id, question_order),
    CONSTRAINT ck_attempt_question_points_non_negative CHECK (points_snapshot >= 0)
);

CREATE TABLE attempt_question_options (
    id BIGSERIAL PRIMARY KEY,
    attempt_question_id BIGINT NOT NULL REFERENCES attempt_questions(id) ON DELETE CASCADE,
    option_order INT NOT NULL,
    content_snapshot TEXT NOT NULL,
    is_correct_snapshot BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_attempt_question_option_order UNIQUE (attempt_question_id, option_order)
);

CREATE TABLE student_answers (
    id BIGSERIAL PRIMARY KEY,
    attempt_question_id BIGINT NOT NULL UNIQUE REFERENCES attempt_questions(id) ON DELETE CASCADE,
    answer_text TEXT,
    is_correct BOOLEAN,
    score_awarded NUMERIC(10,2) NOT NULL DEFAULT 0,
    saved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_at TIMESTAMPTZ,
    graded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_student_answer_score_non_negative CHECK (score_awarded >= 0)
);

CREATE TABLE student_answer_options (
    id BIGSERIAL PRIMARY KEY,
    student_answer_id BIGINT NOT NULL REFERENCES student_answers(id) ON DELETE CASCADE,
    attempt_question_option_id BIGINT NOT NULL REFERENCES attempt_question_options(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_student_answer_option UNIQUE (student_answer_id, attempt_question_option_id)
);

CREATE TABLE attempt_events (
    id BIGSERIAL PRIMARY KEY,
    attempt_id BIGINT NOT NULL REFERENCES exam_attempts(id) ON DELETE CASCADE,
    event_type attempt_event_type_enum NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =========================================================
-- Audit / ads
-- =========================================================

CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(100) NOT NULL,
    entity_id BIGINT,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ads (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    image_url TEXT,
    target_url TEXT,
    placement ad_placement_enum NOT NULL,
    ad_status generic_status_enum NOT NULL DEFAULT 'inactive',
    start_time TIMESTAMPTZ,
    end_time TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_ads_time_window CHECK (start_time IS NULL OR end_time IS NULL OR end_time > start_time)
);

CREATE TABLE ad_impressions (
    id BIGSERIAL PRIMARY KEY,
    ad_id BIGINT NOT NULL REFERENCES ads(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    shown_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- =========================================================
-- V2 additive schema: teacher scope, import persistence, question taxonomy
-- =========================================================

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'teacher_class_permission_enum') THEN
        CREATE TYPE teacher_class_permission_enum AS ENUM ('owner', 'editor', 'viewer');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'question_visibility_enum') THEN
        CREATE TYPE question_visibility_enum AS ENUM ('private', 'school_shared', 'public_readonly');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'attachment_kind_enum') THEN
        CREATE TYPE attachment_kind_enum AS ENUM ('image', 'audio', 'video', 'document', 'other');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'attachment_target_enum') THEN
        CREATE TYPE attachment_target_enum AS ENUM ('question_body', 'option', 'explanation', 'unknown');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ai_provider_enum') THEN
        CREATE TYPE ai_provider_enum AS ENUM ('openai', 'gemini', 'local', 'other');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'ai_run_status_enum') THEN
        CREATE TYPE ai_run_status_enum AS ENUM ('pending', 'running', 'succeeded', 'failed');
    END IF;
END $$;

ALTER TYPE import_parse_status_enum ADD VALUE IF NOT EXISTS 'needs_ocr';
ALTER TYPE import_parse_status_enum ADD VALUE IF NOT EXISTS 'needs_conversion';
ALTER TYPE import_source_type_enum ADD VALUE IF NOT EXISTS 'doc' BEFORE 'docx';

CREATE TABLE IF NOT EXISTS teacher_class_assignments (
    id BIGSERIAL PRIMARY KEY,
    class_id BIGINT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    teacher_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission teacher_class_permission_enum NOT NULL DEFAULT 'editor',
    assignment_status generic_status_enum NOT NULL DEFAULT 'active',
    assigned_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_teacher_class_assignment UNIQUE (class_id, teacher_user_id)
);

ALTER TABLE import_batches
    ADD COLUMN IF NOT EXISTS original_filename VARCHAR(255),
    ADD COLUMN IF NOT EXISTS source_file_url TEXT,
    ADD COLUMN IF NOT EXISTS file_size_bytes BIGINT,
    ADD COLUMN IF NOT EXISTS file_sha256 CHAR(64),
    ADD COLUMN IF NOT EXISTS content_fingerprint CHAR(64),
    ADD COLUMN IF NOT EXISTS parser_model VARCHAR(100),
    ADD COLUMN IF NOT EXISTS document_title TEXT,
    ADD COLUMN IF NOT EXISTS image_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS asset_summary_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS parse_error TEXT;

CREATE TABLE IF NOT EXISTS ai_model_runs (
    id BIGSERIAL PRIMARY KEY,
    import_batch_id BIGINT NOT NULL REFERENCES import_batches(id) ON DELETE CASCADE,
    provider ai_provider_enum NOT NULL DEFAULT 'other',
    model VARCHAR(100) NOT NULL,
    purpose VARCHAR(100) NOT NULL,
    prompt_version VARCHAR(100),
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    request_count INT NOT NULL DEFAULT 1,
    estimated_cost_usd NUMERIC(12,6) NOT NULL DEFAULT 0,
    run_status ai_run_status_enum NOT NULL DEFAULT 'pending',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS subjects (
    id BIGSERIAL PRIMARY KEY,
    subject_code VARCHAR(50) NOT NULL UNIQUE,
    subject_name VARCHAR(255) NOT NULL,
    description TEXT,
    subject_status generic_status_enum NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS topics (
    id BIGSERIAL PRIMARY KEY,
    subject_id BIGINT NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    parent_topic_id BIGINT REFERENCES topics(id) ON DELETE SET NULL,
    topic_code VARCHAR(50),
    topic_name VARCHAR(255) NOT NULL,
    description TEXT,
    topic_status generic_status_enum NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_topic_subject_code UNIQUE (subject_id, topic_code)
);

CREATE TABLE IF NOT EXISTS question_tags (
    id BIGSERIAL PRIMARY KEY,
    tag_name VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE question_bank
    ADD COLUMN IF NOT EXISTS subject_id BIGINT REFERENCES subjects(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS topic_id BIGINT REFERENCES topics(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS default_points NUMERIC(10,2) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS visibility question_visibility_enum NOT NULL DEFAULT 'private';

ALTER TABLE import_items
    ADD COLUMN IF NOT EXISTS source_order INT,
    ADD COLUMN IF NOT EXISTS normalized_question_json JSONB,
    ADD COLUMN IF NOT EXISTS ai_confidence NUMERIC(5,4),
    ADD COLUMN IF NOT EXISTS approved_question_id BIGINT REFERENCES question_bank(id) ON DELETE SET NULL;

CREATE TABLE IF NOT EXISTS question_bank_tags (
    question_id BIGINT NOT NULL REFERENCES question_bank(id) ON DELETE CASCADE,
    tag_id BIGINT NOT NULL REFERENCES question_tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (question_id, tag_id)
);

CREATE TABLE IF NOT EXISTS question_attachments (
    id BIGSERIAL PRIMARY KEY,
    question_id BIGINT NOT NULL REFERENCES question_bank(id) ON DELETE CASCADE,
    attachment_kind attachment_kind_enum NOT NULL DEFAULT 'other',
    file_url TEXT NOT NULL,
    alt_text TEXT,
    display_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS import_item_assets (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES import_batches(id) ON DELETE CASCADE,
    import_item_id BIGINT REFERENCES import_items(id) ON DELETE SET NULL,
    target attachment_target_enum NOT NULL DEFAULT 'unknown',
    option_label VARCHAR(10),
    file_url TEXT NOT NULL,
    mime_type VARCHAR(100),
    page_number INT,
    display_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS question_option_attachments (
    id BIGSERIAL PRIMARY KEY,
    question_option_id BIGINT NOT NULL REFERENCES question_bank_options(id) ON DELETE CASCADE,
    attachment_kind attachment_kind_enum NOT NULL DEFAULT 'image',
    file_url TEXT NOT NULL,
    alt_text TEXT,
    display_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE question_attachments
    ADD COLUMN IF NOT EXISTS target attachment_target_enum NOT NULL DEFAULT 'question_body';

ALTER TABLE exam_attempts
    ADD COLUMN IF NOT EXISTS current_question_order INT,
    ADD COLUMN IF NOT EXISTS last_saved_at TIMESTAMPTZ;

-- =========================================================
-- Indexes
-- =========================================================

CREATE INDEX idx_user_roles_role_id ON user_roles(role_id);
CREATE INDEX idx_classes_homeroom_teacher ON classes(homeroom_teacher_user_id);
CREATE INDEX idx_class_members_student ON class_members(student_user_id);
CREATE INDEX idx_teacher_class_assignments_teacher ON teacher_class_assignments(teacher_user_id);
CREATE INDEX idx_import_items_batch ON import_items(batch_id, review_status);
CREATE INDEX idx_import_batches_file_hash ON import_batches(file_sha256);
CREATE INDEX idx_import_batches_content_fingerprint ON import_batches(content_fingerprint);
CREATE INDEX idx_ai_model_runs_batch ON ai_model_runs(import_batch_id);
CREATE INDEX idx_ai_model_runs_provider_model_created_at ON ai_model_runs(provider, model, created_at);
CREATE INDEX idx_subjects_status ON subjects(subject_status);
CREATE INDEX idx_topics_subject_id ON topics(subject_id);
CREATE INDEX idx_topics_parent_topic_id ON topics(parent_topic_id);
CREATE INDEX idx_question_bank_subject ON question_bank(subject_id);
CREATE INDEX idx_question_bank_topic ON question_bank(topic_id);
CREATE INDEX idx_question_bank_visibility_status ON question_bank(visibility, question_status);
CREATE INDEX idx_question_bank_tags_tag ON question_bank_tags(tag_id);
CREATE INDEX idx_import_item_assets_batch_order ON import_item_assets(batch_id, import_item_id, display_order);
CREATE INDEX idx_question_attachments_question_order ON question_attachments(question_id, display_order);
CREATE INDEX idx_question_option_attachments_option_order ON question_option_attachments(question_option_id, display_order);
CREATE INDEX idx_question_bank_status ON question_bank(question_status);
CREATE INDEX idx_question_bank_creator ON question_bank(created_by_user_id);
CREATE INDEX idx_question_bank_options_question_id ON question_bank_options(question_id);
CREATE INDEX idx_exam_status_window ON exams(exam_status, start_time, end_time);
CREATE INDEX idx_exam_questions_exam_id ON exam_questions(exam_id);
CREATE INDEX idx_exam_targets_class_id ON exam_targets(class_id);
CREATE INDEX idx_exam_versions_exam_id ON exam_versions(exam_id, version_no);
CREATE INDEX idx_exam_version_questions_version_id ON exam_version_questions(exam_version_id);
CREATE INDEX idx_exam_attempts_student ON exam_attempts(student_user_id);
CREATE INDEX idx_exam_attempts_status_end_at ON exam_attempts(attempt_status, end_at);
CREATE INDEX idx_attempt_questions_attempt_id ON attempt_questions(attempt_id);
CREATE INDEX idx_attempt_question_options_attempt_question_id ON attempt_question_options(attempt_question_id);
CREATE INDEX idx_student_answers_saved_at ON student_answers(saved_at);
CREATE INDEX idx_attempt_events_attempt_id_created_at ON attempt_events(attempt_id, created_at);
CREATE INDEX idx_attempt_events_metadata_gin ON attempt_events USING GIN (metadata_json);
CREATE INDEX idx_audit_logs_actor_created_at ON audit_logs(actor_user_id, created_at);
CREATE INDEX idx_audit_logs_metadata_gin ON audit_logs USING GIN (metadata_json);
CREATE INDEX idx_ads_window_status ON ads(ad_status, placement, start_time, end_time);
CREATE INDEX idx_ad_impressions_ad_id ON ad_impressions(ad_id);

-- =========================================================
-- Triggers
-- =========================================================

CREATE TRIGGER trg_users_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_student_profiles_updated_at
BEFORE UPDATE ON student_profiles
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_teacher_profiles_updated_at
BEFORE UPDATE ON teacher_profiles
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_classes_updated_at
BEFORE UPDATE ON classes
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_class_members_updated_at
BEFORE UPDATE ON class_members
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_teacher_class_assignments_updated_at
BEFORE UPDATE ON teacher_class_assignments
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_import_batches_updated_at
BEFORE UPDATE ON import_batches
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_ai_model_runs_updated_at
BEFORE UPDATE ON ai_model_runs
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_import_items_updated_at
BEFORE UPDATE ON import_items
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_subjects_updated_at
BEFORE UPDATE ON subjects
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_topics_updated_at
BEFORE UPDATE ON topics
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_question_bank_updated_at
BEFORE UPDATE ON question_bank
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_question_bank_options_updated_at
BEFORE UPDATE ON question_bank_options
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_exams_updated_at
BEFORE UPDATE ON exams
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_exam_questions_updated_at
BEFORE UPDATE ON exam_questions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_exam_attempts_updated_at
BEFORE UPDATE ON exam_attempts
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_student_answers_updated_at
BEFORE UPDATE ON student_answers
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_ads_updated_at
BEFORE UPDATE ON ads
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- =========================================================
-- Production bootstrap seed
-- =========================================================

INSERT INTO roles (code, name)
VALUES
    ('student', 'Student'),
    ('teacher', 'Teacher'),
    ('admin', 'Administrator')
ON CONFLICT (code) DO UPDATE
SET name = EXCLUDED.name;

-- First production admin account.
-- Username: admin
-- Password: Admin@ExamHub-2026#N7q
-- Change this password immediately after first login.
INSERT INTO users (username, password_hash, account_status)
VALUES ('admin', 'sha256:45972f49ed270f43ec5175b6530e844d1d580cca33ca06d79bf94c6b5dad49b4', 'active')
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

COMMIT;
