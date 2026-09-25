-- SmartFM LMS — Initial Schema
-- Maps to §6 Key Data Models in the architecture document.

-- ─── Extensions ───────────────────────────────────────────────────────────────

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ─── Users ────────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    smartfm_id    UUID,
    name          TEXT NOT NULL,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'employee' CHECK (role IN ('employee', 'instructor', 'admin')),
    avatar_url    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_smartfm_id ON users(smartfm_id) WHERE smartfm_id IS NOT NULL;

-- ─── Courses ──────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS courses (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    created_by       UUID NOT NULL REFERENCES users(id),
    title            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    category         TEXT NOT NULL DEFAULT '',
    level            TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'ready', 'archived')),
    hls_url          TEXT,
    thumbnail_url    TEXT,
    duration_seconds INT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_courses_status ON courses(status);
CREATE INDEX IF NOT EXISTS idx_courses_created_by ON courses(created_by);

-- ─── Modules ──────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS modules (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    course_id        UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    title            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    duration_seconds INT NOT NULL DEFAULT 0,
    order_index      INT NOT NULL DEFAULT 0,
    hls_url          TEXT,
    status           TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'ready', 'archived')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_modules_course ON modules(course_id);

-- ─── Assessment Results ───────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS assessment_results (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    lms_user_id    UUID NOT NULL REFERENCES users(id),
    course_id      UUID NOT NULL REFERENCES courses(id),
    score          INT NOT NULL CHECK (score >= 0 AND score <= 100),
    passed         BOOLEAN NOT NULL DEFAULT FALSE,
    attempt_number INT NOT NULL DEFAULT 1,
    completed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_assessment_user ON assessment_results(lms_user_id);
CREATE INDEX IF NOT EXISTS idx_assessment_course ON assessment_results(course_id);

-- ─── Quiz Questions ───────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS quiz_questions (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    course_id         UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    text              TEXT NOT NULL,
    options           JSONB NOT NULL DEFAULT '[]',
    correct_option_id TEXT NOT NULL,
    explanation       TEXT NOT NULL DEFAULT '',
    order_index       INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_quiz_course ON quiz_questions(course_id);

-- ─── Content Library ──────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS content_items (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title          TEXT NOT NULL,
    type           TEXT NOT NULL CHECK (type IN ('video', 'assessment', 'test')),
    category       TEXT NOT NULL DEFAULT '',
    duration       INT,
    question_count INT,
    status         TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    created_by     TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS categories (
    id   UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL UNIQUE
);

-- ─── Employee Profiles (enriched view) ────────────────────────────────────────
-- This is a materialized view / denormalized table maintained by the User Service.

CREATE TABLE IF NOT EXISTS employee_profiles (
    id                UUID PRIMARY KEY,
    name              TEXT NOT NULL,
    employee_id       TEXT NOT NULL UNIQUE,
    role              TEXT NOT NULL,
    department        TEXT NOT NULL DEFAULT '',
    site              TEXT NOT NULL DEFAULT '',
    joining_date      TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'at_risk', 'inactive')),
    courses_assigned  INT NOT NULL DEFAULT 0,
    courses_completed INT NOT NULL DEFAULT 0,
    compliance_status TEXT NOT NULL DEFAULT 'pending' CHECK (compliance_status IN ('compliant', 'pending', 'overdue')),
    avg_score         INT NOT NULL DEFAULT 0,
    total_learning_time FLOAT NOT NULL DEFAULT 0,
    last_active       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Seed users for development ───────────────────────────────────────────────
-- Admin: admin@ismart.com / admin123
-- Learner: learner@ismart.com / learner123

INSERT INTO users (id, name, email, password_hash, role)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'Admin User',
    'admin@ismart.com',
    '$2a$10$/66Mqcx9CuO6zR5Y1VwW8O2nTMRxSZHtCNqv8e.RhCUexKI/CApLq',
    'admin'
) ON CONFLICT (email) DO NOTHING;

INSERT INTO users (id, name, email, password_hash, role)
VALUES (
    'a0000000-0000-0000-0000-000000000002',
    'Test Learner',
    'learner@ismart.com',
    '$2a$10$90/8VzAM9WXmFCGF40kBL.c5YRpK/CiapiHguF.2Mlw46ITPwzIDq',
    'employee'
) ON CONFLICT (email) DO NOTHING;
