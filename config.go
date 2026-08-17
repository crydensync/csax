package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"

	"github.com/crydensync/cryden/v2"
	"github.com/crydensync/cryden/v2/store/postgres"
)

type csaxConfig struct {
	DatabaseURL   string
	JWTSecret     string
	MigrationsDir string
}

func loadConfig() (csaxConfig, error) {
	loadEnvFile(".env")

	cfg := csaxConfig{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		MigrationsDir: os.Getenv("MIGRATIONS_DIR"),
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
