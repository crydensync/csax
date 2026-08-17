package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/crydensync/cryden/v2/store/postgres"
)

func cmdAuditTail(db *sql.DB, email string, limit int) {
	if email == "" {
		fmt.Println("usage: csax audit tail --user <email> [--limit N]")
		os.Exit(1)
	}
	ctx := context.Background()
	userID, err := resolveUserID(db, email)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	as := postgres.NewAuditStore(db)
	events, err := as.ListByUser(ctx, userID, limit)
	if err != nil {
		fmt.Printf("failed to fetch audit log: %v\n", err)
		os.Exit(1)
	}
	if len(events) == 0 {
		fmt.Println("No audit events found.")
		return
	}
	for _, e := range events {
		marker := ""
		if e.Type == "token_reuse_detected" {
			marker = "  ⚠"
		}
		fmt.Printf("%s  %-24s ip=%s%s\n", e.CreatedAt.Format("2006-01-02 15:04"), e.Type, e.IP, marker)
	}
}
