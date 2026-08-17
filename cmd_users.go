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
