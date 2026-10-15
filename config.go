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
	FrontendURL        string
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

	// API-client mode — reach a deployed api instance over HTTP,
	// authenticated as an operator, instead of connecting to Postgres
	// directly. See apiclient.go. Unset (the default) leaves every
	// command working exactly as it does today.
	APIURL      string
	APIEmail    string
	APIPassword string // optional; prompted interactively if unset — never require it in .env in plaintext
}

func loadConfig() (csaxConfig, error) {
	loadEnvFile(".env")

	cfg := csaxConfig{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		MigrationsDir: os.Getenv("MIGRATIONS_DIR"),

		BaseURL:            strings.TrimRight(os.Getenv("BASE_URL"), "/"),
		FrontendURL:        strings.TrimRight(os.Getenv("FRONTEND_URL"), "/"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),

		AIProvider:    os.Getenv("AI_PROVIDER"),
		AIAPIKeyEnv:   os.Getenv("AI_API_KEY_ENV"),
		AIModel:       os.Getenv("AI_MODEL"),
		ReadOnlyDBURL: os.Getenv("READONLY_DATABASE_URL"),

		APIURL:      strings.TrimRight(os.Getenv("CSAX_API_URL"), "/"),
		APIEmail:    os.Getenv("CSAX_API_EMAIL"),
		APIPassword: os.Getenv("CSAX_API_PASSWORD"),
	}
	if cfg.MigrationsDir == "" {
		cfg.MigrationsDir = "./migrations"
	}
	// DATABASE_URL is only required for direct-DB mode. With
	// CSAX_API_URL set, a command reaches the deployment over HTTP as
	// an operator and never needs a database credential at all — that
	// is the entire point of API-client mode, so this check must not
	// apply when it's in use. connectDB/buildEngine still enforce
	// DATABASE_URL for the direct-DB commands that always need it
	// (csax migrate, and any command run without CSAX_API_URL set).
	if cfg.DatabaseURL == "" && cfg.APIURL == "" {
		return cfg, fmt.Errorf("DATABASE_URL is required for direct-DB mode — set it in .env, or set CSAX_API_URL to use API-client mode instead")
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
