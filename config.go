package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"

	"github.com/crydensync/cryden/v2"
	"github.com/crydensync/cryden/v2/store/postgres"
)

type csaxConfig struct {
	DatabaseURL   string
	JWTSecret     string
	MigrationsDir string

	// OAuth — deliberately the SAME env var names api's config uses,
	// so `csax oauth test` checks the actual values production uses,
	// not a separate csax-only copy that could drift out of sync.
	BaseURL            string
	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string

	// AI — all optional. ai commands fail with a clear message if
	// these aren't set, rather than csax refusing to start at all.
	AIProvider    string // e.g. "openrouter"
	AIAPIKeyEnv   string // name of the env var holding the API key — never the key itself, so it's not persisted in .env in plaintext by `config init`
	AIModel       string
	ReadOnlyDBURL string // separate connection string, MUST point at a read-only Postgres role — this is the real safety boundary, not just ai.validateIntent
}

func loadConfig() (csaxConfig, error) {
	loadEnvFile(".env")

	cfg := csaxConfig{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		MigrationsDir: os.Getenv("MIGRATIONS_DIR"),

		BaseURL:            strings.TrimRight(os.Getenv("BASE_URL"), "/"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),

		AIProvider:    os.Getenv("AI_PROVIDER"),
		AIAPIKeyEnv:   os.Getenv("AI_API_KEY_ENV"),
		AIModel:       os.Getenv("AI_MODEL"),
		ReadOnlyDBURL: os.Getenv("READONLY_DATABASE_URL"),
	}
	if cfg.MigrationsDir == "" {
		cfg.MigrationsDir = "./migrations"
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required — set it in .env or run `csax config init`")
	}
	return cfg, nil
}

// connectDB opens the DB connection every command needs.
func connectDB(cfg csaxConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("could not reach database: %w", err)
	}
	return db, nil
}

// buildEngine wires a cryden.Engine — used by users/sessions/audit
// commands, which need real engine logic (ownership checks, lockout
// semantics), not raw SQL. Migrate/health commands don't need this,
// they work at the plain-DB level.
func buildEngine(cfg csaxConfig, db *sql.DB) (*cryden.Engine, error) {
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required for this command — set it in .env")
	}
	return cryden.New(cryden.Config{
		JWTSecret:     cfg.JWTSecret,
		Users:         postgres.NewUserStore(db),
		Sessions:      postgres.NewSessionStore(db),
		Audit:         postgres.NewAuditStore(db),
		Verifications: postgres.NewVerificationStore(db),
	})
}
