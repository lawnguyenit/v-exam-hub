package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"website-exam/internal/authsession"
	"website-exam/internal/config"
)

func connectDB(ctx context.Context, cfg config.Runtime) (*pgxpool.Pool, error) {
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		if cfg.IsProduction() {
			return nil, fmt.Errorf("DB_URL is required when APP_ENV=production")
		}
		return nil, fmt.Errorf("DB_URL is empty")
	}
	startupTimeout := cfg.DBStartupTimeout

	deadlineCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	var lastErr error
	for attempt := 1; ; attempt++ {
		attemptCtx, attemptCancel := context.WithTimeout(deadlineCtx, 5*time.Second)
		db, err := pgxpool.New(attemptCtx, cfg.DatabaseURL)
		if err == nil {
			err = db.Ping(attemptCtx)
		}
		attemptCancel()

		if err == nil {
			if err := ensureCoreSchema(deadlineCtx, db); err != nil {
				db.Close()
				return nil, err
			}
			if err := ensureDatabaseCompatibility(deadlineCtx, db); err != nil {
				db.Close()
				return nil, err
			}
			if err := authsession.EnsureSchema(deadlineCtx, db); err != nil {
				db.Close()
				return nil, err
			}
			log.Println("Database connected")
			return db, nil
		}

		lastErr = err
		if deadlineCtx.Err() != nil {
			break
		}
		log.Printf("database not ready (attempt %d): %v", attempt, err)
		select {
		case <-time.After(2 * time.Second):
		case <-deadlineCtx.Done():
		}
	}

	if deadlineCtx.Err() != nil && lastErr != nil {
		return nil, fmt.Errorf("timed out waiting for database after %s: %w", startupTimeout, lastErr)
	}
	return nil, lastErr
}

func ensureCoreSchema(ctx context.Context, db *pgxpool.Pool) error {
	requiredTables := []string{
		"roles",
		"users",
		"user_roles",
		"student_profiles",
		"teacher_profiles",
		"classes",
		"class_members",
		"import_batches",
		"import_items",
		"question_bank",
		"question_bank_options",
		"exams",
		"exam_questions",
		"exam_targets",
		"exam_attempts",
	}

	missing := make([]string, 0, len(requiredTables))
	for _, tableName := range requiredTables {
		exists, err := tableExists(ctx, db, tableName)
		if err != nil {
			return err
		}
		if !exists {
			missing = append(missing, tableName)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"missing core schema tables: %s. Bootstrap PostgreSQL báº±ng file D:\\v-exam-hub\\v-exam-hub-local-db.session.sql hoáº·c mount file nÃ y vÃ o /docker-entrypoint-initdb.d trÆ°á»›c khi cháº¡y backend",
			strings.Join(missing, ", "),
		)
	}
	return nil
}

func tableExists(ctx context.Context, db *pgxpool.Pool, tableName string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, tableName).Scan(&exists)
	return exists, err
}

func ensureDatabaseCompatibility(ctx context.Context, db *pgxpool.Pool) error {
	// 1. Khá»Ÿi táº¡o cÃ¡c Type (ENUM) náº¿u chÆ°a cÃ³
	// ChÃºng ta bá»c trong khá»‘i DO Ä‘á»ƒ trÃ¡nh lá»—i "already exists"
	setupEnumsSQL := `
        DO $$ BEGIN
            IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'attachment_target_enum') THEN
                CREATE TYPE attachment_target_enum AS ENUM ('question_body', 'option', 'explanation', 'unknown');
            END IF;
        END $$;
    `
	if _, err := db.Exec(ctx, setupEnumsSQL); err != nil {
		return err
	}

	// 2. Cáº­p nháº­t ENUM hiá»‡n cÃ³ (ThÃªm giÃ¡ trá»‹ má»›i náº¿u cáº§n)
	// Lá»‡nh ADD VALUE IF NOT EXISTS chá»‰ cháº¡y Ä‘Æ°á»£c tá»« Postgres 13+
	if _, err := db.Exec(ctx, `ALTER TYPE exam_mode_enum ADD VALUE IF NOT EXISTS 'official'`); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `ALTER TYPE exam_mode_enum ADD VALUE IF NOT EXISTS 'attendance'`); err != nil {
		return err
	}

	// 3. Thá»±c thi cÃ¡c lá»‡nh táº¡o báº£ng vÃ  chá»‰nh sá»­a rÃ ng buá»™c (Constraint)
	// LÆ°u Ã½: CÃ¡c báº£ng nhÆ° 'exams' pháº£i tá»“n táº¡i trÆ°á»›c khi cháº¡y ALTER TABLE
	_, err := db.Exec(ctx, `
        -- Chá»‰ cháº¡y ALTER TABLE náº¿u báº£ng 'exams' Ä‘Ã£ tá»“n táº¡i
        DO $$ BEGIN
            IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'exams') THEN
                ALTER TABLE exams DROP CONSTRAINT IF EXISTS ck_exam_attempts_positive;
                ALTER TABLE exams DROP CONSTRAINT IF EXISTS ck_exam_attempts_non_negative;
                ALTER TABLE exams ADD CONSTRAINT ck_exam_attempts_non_negative CHECK (max_attempts_per_student >= 0);
            END IF;
        END $$;

        -- Táº¡o báº£ng má»›i náº¿u chÆ°a cÃ³
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

        CREATE INDEX IF NOT EXISTS idx_import_item_assets_batch_order ON import_item_assets(batch_id, import_item_id, display_order);
    `)
	return err
}
