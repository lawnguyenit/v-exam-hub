package config

import (
	"os"
	"strings"
	"time"
)

const defaultLocalDatabaseURL = "postgres://admin:123@localhost:5432/v_exam_hub?sslmode=disable"

type Runtime struct {
	AppEnv             string
	Address            string
	DatabaseURL        string
	DBStartupTimeout   time.Duration
	CORSAllowedOrigins []string
}

func Load() Runtime {
	appEnv := strings.TrimSpace(os.Getenv("APP_ENV"))
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	return Runtime{
		AppEnv:             appEnv,
		Address:            ":" + port,
		DatabaseURL:        databaseURL(appEnv),
		DBStartupTimeout:   durationEnv("DB_STARTUP_TIMEOUT", 90*time.Second),
		CORSAllowedOrigins: corsAllowedOrigins(appEnv),
	}
}

func (c Runtime) IsProduction() bool {
	return strings.EqualFold(strings.TrimSpace(c.AppEnv), "production")
}

func databaseURL(appEnv string) string {
	if value := strings.TrimSpace(os.Getenv("DB_URL")); value != "" {
		return value
	}
	if strings.EqualFold(strings.TrimSpace(appEnv), "production") {
		return ""
	}
	return defaultLocalDatabaseURL
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func corsAllowedOrigins(appEnv string) []string {
	values := csvEnv("CORS_ALLOWED_ORIGINS")
	if len(values) > 0 {
		return values
	}
	if strings.EqualFold(strings.TrimSpace(appEnv), "production") {
		return nil
	}
	return []string{"*"}
}

func csvEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}
