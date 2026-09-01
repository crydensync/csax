package main

import (
	"database/sql"
	"fmt"
)

// cmdDoctor runs every health/config check csax knows how to run, in
// one command. Reuses the same underlying checks as `csax health` and
// `csax ai audit` rather than re-implementing them — one source of
// truth for "is JWT_SECRET long enough", not two that could drift.
func cmdDoctor(cfg csaxConfig, db *sql.DB) {
	fmt.Println("csax doctor")
	fmt.Println()

	fmt.Println("Database")
	if err := db.Ping(); err != nil {
		fmt.Printf("  ✗ unreachable: %v\n", err)
	} else {
		fmt.Println("  ✔ reachable")
		checkMigration(db, "csax_migrations", "run `csax migrate up`")
		checkMigration(db, "oauth_identities", "run the 0002_oauth_identities migration (see README)")
	}

	fmt.Println()
	fmt.Println("AI-assisted commands")
	for _, f := range runAuditChecklist(cfg) {
		icon, color := "✓", green
		if f.Severity == "HIGH" {
			icon, color = "✗", red
		} else if f.Severity == "MEDIUM" {
			icon, color = "⚠", yellow
		}
		fmt.Printf("  %s %s\n", color(icon), f.Message)
	}
	if _, err := newLLMProvider(cfg); err != nil {
		fmt.Printf("  ⚠ %v (run `csax ai config`)\n", err)
	} else {
		fmt.Println("  ✔ AI provider configured")
	}

	fmt.Println()
	fmt.Println("OAuth")
	anyConfigured := false
	for _, p := range oauthProviders(cfg) {
		configured := p.ClientIDSet && p.ClientSecretSet
		if configured {
			anyConfigured = true
			fmt.Printf("  ✔ %s configured\n", p.Name)
		}
	}
	if !anyConfigured {
		fmt.Println("  ⚠ no providers configured (optional — run `csax oauth config` to add one)")
	}
	if cfg.BaseURL == "" && anyConfigured {
		fmt.Println("  ✗ BASE_URL is not set, but a provider is configured — OAuth callback URLs cannot be built")
	}

	fmt.Println()
}

// checkMigration reports whether a table exists — a cheap, reliable
// proxy for "has this migration been run" without needing a real
// migration-tracking scheme for every possible table.
func checkMigration(db *sql.DB, table, fixHint string) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1`, table).Scan(&count)
	if err != nil || count == 0 {
		fmt.Printf("  ✗ table %q missing — %s\n", table, fixHint)
		return
	}
	fmt.Printf("  ✔ table %q present\n", table)
}
