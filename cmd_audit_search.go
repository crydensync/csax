package main

import (
	"database/sql"
	"fmt"
	"os"
)

// cmdAuditSearch queries across ALL users by event type — the
// engine's AuditStore interface only supports ListByUser (per-user),
// so a system-wide search follows the same direct-SQL pattern as
// cmdStats, rather than adding a new engine interface method for it.
func cmdAuditSearch(db *sql.DB, eventType string, limit int) {
	if eventType == "" {
		fmt.Println("usage: csax audit search --event <type> [--limit N]")
		fmt.Println("example event types: login_success, login_failed, token_reuse_detected, account_locked")
		os.Exit(1)
	}

	rows, err := db.Query(`
		SELECT ae.type, ae.ip, ae.created_at, u.email
		FROM audit_events ae
		LEFT JOIN users u ON u.id = ae.user_id
		WHERE ae.type = $1
		ORDER BY ae.created_at DESC
		LIMIT $2
	`, eventType, limit)
	if err != nil {
		fmt.Printf("query failed: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var evType, ip, email sql.NullString
		var createdAt string
		if err := rows.Scan(&evType, &ip, &createdAt, &email); err != nil {
			fmt.Printf("scan failed: %v\n", err)
			os.Exit(1)
		}
		found = true

		emailStr := "(no user)"
		if email.Valid {
			emailStr = email.String
		}

		line := fmt.Sprintf("%s  %-24s user=%s ip=%s", createdAt, evType.String, emailStr, ip.String)
		if evType.String == "token_reuse_detected" || evType.String == "account_locked" {
			fmt.Println(yellow(line + "  ⚠"))
		} else {
			fmt.Println(line)
		}
	}
	if !found {
		fmt.Println(dim("No matching events found."))
	}
}
