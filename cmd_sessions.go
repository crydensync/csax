package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/crydensync/cryden/v2"
	"github.com/crydensync/cryden/v2/store/postgres"
)

func cmdSessionsList(db *sql.DB, email string, jsonOutput bool) {
	if email == "" {
		fmt.Println("usage: csax sessions list --user <email> [--json]")
		os.Exit(1)
	}
	ctx := context.Background()
	us := postgres.NewUserStore(db)
	user, err := us.GetByEmail(ctx, email)
	if err != nil {
		fmt.Println(red("failed to find user: " + err.Error()))
		os.Exit(1)
	}

	ss := postgres.NewSessionStore(db)
	sessions, err := ss.ListByUser(ctx, user.ID)
	if err != nil {
		fmt.Printf("failed to list sessions: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		type sessionOut struct {
			ID        string `json:"id"`
			IP        string `json:"ip"`
			UserAgent string `json:"user_agent"`
			CreatedAt string `json:"created_at"`
		}
		out := make([]sessionOut, 0, len(sessions))
		for _, s := range sessions {
			out = append(out, sessionOut{ID: s.ID, IP: s.IP, UserAgent: s.UserAgent, CreatedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z07:00")})
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return
	}

	if len(sessions) == 0 {
		fmt.Println(dim("No active sessions."))
		return
	}
	for _, s := range sessions {
		fmt.Printf("%-38s %-20s %-16s %s\n", s.ID, s.UserAgent, s.IP, s.CreatedAt.Format("2006-01-02 15:04"))
	}
}

// cmdSessionsRevoke uses the real engine (cryden.RevokeSession), not a
// raw store call — the ownership check (session must belong to the
// given user) is real engine logic, not something the CLI should
// reimplement or bypass.
func cmdSessionsRevoke(db *sql.DB, engine *cryden.Engine, email, sessionID string) {
	if email == "" || sessionID == "" {
		fmt.Println("usage: csax sessions revoke <session-id> --user <email>")
		os.Exit(1)
	}
	ctx := context.Background()

	userID, err := resolveUserID(db, email)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := cryden.RevokeSession(ctx, engine, sessionID, userID); err != nil {
		fmt.Println(red("failed to revoke session: " + err.Error()))
		os.Exit(1)
	}
	fmt.Println(green("✔ Revoked session " + sessionID))
}

func cmdSessionsRevokeAll(db *sql.DB, engine *cryden.Engine, email string) {
	if email == "" {
		fmt.Println("usage: csax sessions revoke-all --user <email>")
		os.Exit(1)
	}
	ctx := context.Background()
	userID, err := resolveUserID(db, email)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := cryden.LogoutAll(ctx, engine, userID); err != nil {
		fmt.Println(red("failed to revoke sessions: " + err.Error()))
		os.Exit(1)
	}
	fmt.Println(green("✔ Revoked all sessions for " + email))
}
