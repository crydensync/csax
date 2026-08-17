package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
)

// cmdUsersList queries all users directly via SQL — UserStore has no
// ListAll method (a deliberate v1/v2 scope boundary, not an
// oversight; see docs/cli/index.mdx), so this follows the same
// direct-SQL pattern already used by cmdStats and cmdAuditSearch.
// If/when the engine gains a real ListAll method, this should be
// simplified to call it instead of querying the schema directly —
// but the behavior and output shape here won't need to change.
func cmdUsersList(db *sql.DB, limit, offset int, jsonOutput bool) {
	rows, err := db.Query(`
		SELECT id, email, created_at, locked_until
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		fmt.Println(red("query failed: " + err.Error()))
		os.Exit(1)
	}
	defer rows.Close()

	type userRow struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		CreatedAt string `json:"created_at"`
		Locked    bool   `json:"locked"`
	}
	var users []userRow

	for rows.Next() {
		var id, email, createdAt string
		var lockedUntil sql.NullString
		if err := rows.Scan(&id, &email, &createdAt, &lockedUntil); err != nil {
			fmt.Println(red("scan failed: " + err.Error()))
			os.Exit(1)
		}
		users = append(users, userRow{ID: id, Email: email, CreatedAt: createdAt, Locked: lockedUntil.Valid})
	}

	if jsonOutput {
		if users == nil {
			users = []userRow{}
		}
		b, _ := json.MarshalIndent(users, "", "  ")
		fmt.Println(string(b))
		return
	}

	if len(users) == 0 {
		fmt.Println(dim("No users found."))
		return
	}
	for _, u := range users {
		lockLabel := ""
		if u.Locked {
			lockLabel = yellow(" [locked]")
		}
		fmt.Printf("%s  %-40s %s%s\n", u.ID, u.Email, u.CreatedAt, lockLabel)
	}
	fmt.Println(dim(fmt.Sprintf("\nShowing %d (limit=%d offset=%d) — use --limit/--offset to page.", len(users), limit, offset)))
}
