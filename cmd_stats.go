package main

import (
	"database/sql"
	"fmt"
	"os"
)

// cmdStats queries aggregate counts directly via SQL — the engine's
// store interfaces have no "count all" methods (by design; see
// docs/cli/index.mdx on the users-list scope boundary), so this
// follows the same pattern csax's own migrate commands already use:
// direct SQL against the known, documented schema, not a change to
// the engine's Go code.
func cmdStats(db *sql.DB) {
	var totalUsers, activeSessions, signupsToday, failedLoginsToday, lockedAccounts int

	must := func(err error, label string) {
		if err != nil {
			fmt.Printf("failed to query %s: %v\n", label, err)
			os.Exit(1)
		}
	}

	must(db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&totalUsers), "total users")
	must(db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE revoked_at IS NULL`).Scan(&activeSessions), "active sessions")
	must(db.QueryRow(`SELECT COUNT(*) FROM users WHERE created_at >= CURRENT_DATE`).Scan(&signupsToday), "signups today")
	must(db.QueryRow(`SELECT COUNT(*) FROM audit_events WHERE type = 'login_failed' AND created_at >= CURRENT_DATE`).Scan(&failedLoginsToday), "failed logins today")
	must(db.QueryRow(`SELECT COUNT(*) FROM users WHERE locked_until IS NOT NULL AND locked_until > now()`).Scan(&lockedAccounts), "locked accounts")

	fmt.Println(dim("CrydenSync — system stats"))
	fmt.Printf("Total users:          %d\n", totalUsers)
	fmt.Printf("Active sessions:      %d\n", activeSessions)
	fmt.Printf("Signups today:        %d\n", signupsToday)
	fmt.Printf("Failed logins today:  %d\n", failedLoginsToday)
	if lockedAccounts > 0 {
		fmt.Println(yellow(fmt.Sprintf("Locked accounts:      %d", lockedAccounts)))
	} else {
		fmt.Printf("Locked accounts:      %d\n", lockedAccounts)
	}
}
