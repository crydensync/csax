package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const trackingTableSQL = `
CREATE TABLE IF NOT EXISTS csax_migrations (
    filename   TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

func ensureTrackingTable(db *sql.DB) error {
	_, err := db.Exec(trackingTableSQL)
	return err
}

func appliedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT filename FROM csax_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

func listMigrationFiles(dir, suffix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading migrations dir %q: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), suffix) {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func cmdMigrateUp(cfg csaxConfig, db *sql.DB) {
	if err := ensureTrackingTable(db); err != nil {
		fmt.Printf("failed to set up tracking table: %v\n", err)
		os.Exit(1)
	}
	applied, err := appliedMigrations(db)
	if err != nil {
		fmt.Printf("failed to read applied migrations: %v\n", err)
		os.Exit(1)
	}
	files, err := listMigrationFiles(cfg.MigrationsDir, ".up.sql")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	ranAny := false
	for _, f := range files {
		if applied[f] {
			continue
		}
		ranAny = true
		fmt.Printf("Applying %s... ", f)
		sqlBytes, err := os.ReadFile(filepath.Join(cfg.MigrationsDir, f))
		if err != nil {
			fmt.Printf("FAILED (read error: %v)\n", err)
			os.Exit(1)
		}

		tx, err := db.Begin()
		if err != nil {
			fmt.Printf("FAILED (%v)\n", err)
			os.Exit(1)
		}
		if _, err := tx.Exec(string(sqlBytes)); err != nil {
			tx.Rollback()
			fmt.Printf("FAILED (%v)\n", err)
			os.Exit(1)
		}
		if _, err := tx.Exec(`INSERT INTO csax_migrations (filename) VALUES ($1)`, f); err != nil {
			tx.Rollback()
			fmt.Printf("FAILED (recording migration: %v)\n", err)
			os.Exit(1)
		}
		if err := tx.Commit(); err != nil {
			fmt.Printf("FAILED (%v)\n", err)
			os.Exit(1)
		}
		fmt.Println("done")
	}
	if !ranAny {
		fmt.Println("Nothing to apply — already up to date.")
	}
}

func cmdMigrateStatus(cfg csaxConfig, db *sql.DB) {
	if err := ensureTrackingTable(db); err != nil {
		fmt.Printf("failed to set up tracking table: %v\n", err)
		os.Exit(1)
	}
	applied, err := appliedMigrations(db)
	if err != nil {
		fmt.Printf("failed to read applied migrations: %v\n", err)
		os.Exit(1)
	}
	files, err := listMigrationFiles(cfg.MigrationsDir, ".up.sql")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, f := range files {
		if applied[f] {
			fmt.Printf("✔ %s  applied\n", f)
		} else {
			fmt.Printf("✗ %s  pending\n", f)
		}
	}
}

func cmdMigrateDown(cfg csaxConfig, db *sql.DB) {
	if err := ensureTrackingTable(db); err != nil {
		fmt.Printf("failed to set up tracking table: %v\n", err)
		os.Exit(1)
	}

	var lastFile string
	err := db.QueryRow(`SELECT filename FROM csax_migrations ORDER BY applied_at DESC LIMIT 1`).Scan(&lastFile)
	if err == sql.ErrNoRows {
		fmt.Println("No applied migrations to roll back.")
		return
	}
	if err != nil {
		fmt.Printf("failed to find last migration: %v\n", err)
		os.Exit(1)
	}

	downFile := strings.TrimSuffix(lastFile, ".up.sql") + ".down.sql"
	sqlBytes, err := os.ReadFile(filepath.Join(cfg.MigrationsDir, downFile))
	if err != nil {
		fmt.Printf("failed to read %s: %v\n", downFile, err)
		os.Exit(1)
	}

	fmt.Printf("Rolling back %s... ", lastFile)
	tx, err := db.Begin()
	if err != nil {
		fmt.Printf("FAILED (%v)\n", err)
		os.Exit(1)
	}
	if _, err := tx.Exec(string(sqlBytes)); err != nil {
		tx.Rollback()
		fmt.Printf("FAILED (%v)\n", err)
		os.Exit(1)
	}
	if _, err := tx.Exec(`DELETE FROM csax_migrations WHERE filename = $1`, lastFile); err != nil {
		tx.Rollback()
		fmt.Printf("FAILED (recording rollback: %v)\n", err)
		os.Exit(1)
	}
	if err := tx.Commit(); err != nil {
		fmt.Printf("FAILED (%v)\n", err)
		os.Exit(1)
	}
	fmt.Println("done")
}
