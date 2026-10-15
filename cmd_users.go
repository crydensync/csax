package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/crydensync/cryden/v2/store/postgres"
)

func cmdUsersGet(db *sql.DB, email string, jsonOutput bool) {
	if email == "" {
		fmt.Println("usage: csax users get <email> [--json]")
		os.Exit(1)
	}
	us := postgres.NewUserStore(db)
	user, err := us.GetByEmail(context.Background(), email)
	if err != nil {
		fmt.Println(red("failed to find user: " + err.Error()))
		os.Exit(1)
	}

	ss := postgres.NewSessionStore(db)
	sessions, err := ss.ListByUser(context.Background(), user.ID)
	if err != nil {
		fmt.Printf("warning: could not fetch sessions: %v\n", err)
	}

	if jsonOutput {
		out := map[string]any{
			"id":              user.ID,
			"email":           user.Email,
			"created_at":      user.CreatedAt,
			"locked":          user.LockedUntil != nil,
			"locked_until":    user.LockedUntil,
			"failed_attempts": user.FailedAttempts,
			"active_sessions": len(sessions),
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Printf("ID:       %s\n", user.ID)
	fmt.Printf("Email:    %s\n", user.Email)
	fmt.Printf("Created:  %s\n", user.CreatedAt.Format("2006-01-02 15:04"))
	if user.LockedUntil != nil {
		fmt.Println(yellow(fmt.Sprintf("Locked:   yes, until %s", user.LockedUntil.Format("2006-01-02 15:04"))))
	} else {
		fmt.Printf("Locked:   no (failed attempts: %d)\n", user.FailedAttempts)
	}
	fmt.Printf("Sessions: %d active\n", len(sessions))
}

// oauthIdentityRow mirrors one row of oauth_identities — kept local
// to this file since it's a read-only query result, not a store type.
type oauthIdentityRow struct {
	ID         string
	Provider   string
	ExternalID string
	CreatedAt  string
}

// cmdOAuthUsersGet lists which providers an account has linked.
// oauth_identities has no engine store method exposed for this yet
// (see the eight-gaps-style facade decision) — same pattern as
// cmdStats: direct SQL against the known, documented schema, not an
// engine change.
func cmdOAuthUsersGet(db *sql.DB, email string, jsonOutput bool) {
	if email == "" {
		fmt.Println("usage: csax oauth users get <email> [--json]")
		os.Exit(1)
	}

	var userID string
	err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, email).Scan(&userID)
	if err != nil {
		fmt.Println(red("failed to find user: " + err.Error()))
		os.Exit(1)
	}

	rows, err := db.Query(`
		SELECT id, provider, external_id, created_at
		FROM oauth_identities WHERE user_id = $1
		ORDER BY created_at ASC
	`, userID)
	if err != nil {
		fmt.Println(red("failed to query oauth_identities: " + err.Error()))
		os.Exit(1)
	}
	defer rows.Close()

	var identities []oauthIdentityRow
	for rows.Next() {
		var r oauthIdentityRow
		if err := rows.Scan(&r.ID, &r.Provider, &r.ExternalID, &r.CreatedAt); err != nil {
			fmt.Println(red("failed reading row: " + err.Error()))
			os.Exit(1)
		}
		identities = append(identities, r)
	}

	if jsonOutput {
		out := map[string]any{"email": email, "user_id": userID, "linked_providers": identities}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Printf("%s (user_id: %s)\n", email, userID)
	if len(identities) == 0 {
		fmt.Println(dim("  no linked providers"))
		return
	}
	for _, id := range identities {
		fmt.Printf("  %-8s — linked %s\n", id.Provider, id.CreatedAt)
	}
}

// cmdOAuthUnlink force-unlinks one provider from an account — the
// admin escape hatch, same spirit as `csax users unlock`.
func cmdOAuthUnlink(db *sql.DB, email, provider string) {
	if email == "" || provider == "" {
		fmt.Println("usage: csax oauth unlink <email> --provider <name>")
		os.Exit(1)
	}

	var userID string
	err := db.QueryRow(`SELECT id FROM users WHERE email = $1`, email).Scan(&userID)
	if err != nil {
		fmt.Println(red("failed to find user: " + err.Error()))
		os.Exit(1)
	}

	result, err := db.Exec(`DELETE FROM oauth_identities WHERE user_id = $1 AND provider = $2`, userID, provider)
	if err != nil {
		fmt.Println(red("failed to unlink: " + err.Error()))
		os.Exit(1)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		fmt.Println(yellow(fmt.Sprintf("%s has no linked %s account — nothing to do.", email, provider)))
		return
	}
	fmt.Println(green(fmt.Sprintf("✔ Unlinked %s from %s", provider, email)))
}

func cmdUsersUnlock(db *sql.DB, email string) {
	if email == "" {
		fmt.Println("usage: csax users unlock <email>")
		os.Exit(1)
	}
	us := postgres.NewUserStore(db)
	ctx := context.Background()

	user, err := us.GetByEmail(ctx, email)
	if err != nil {
		fmt.Printf("failed to find user: %v\n", err)
		os.Exit(1)
	}

	// ResetFailedAttempts clears both the failed-attempt counter and
	// LockedUntil together — that's the existing engine behavior we're
	// deliberately reusing as-is, per the "no engine changes" scope
	// for this CLI release.
	if err := us.ResetFailedAttempts(ctx, user.ID); err != nil {
		fmt.Printf("failed to unlock: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(green("✔ Unlocked " + email))
}
