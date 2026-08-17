package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/crydensync/cryden/v2/store/postgres"
)

// resolveUserID looks up a user's ID by email via the store layer
// directly — cryden.go's public facade deliberately has no
// email-lookup function (SignUp/Login are the only user-facing entry
// points), so the CLI reaches the store directly for this admin
// convenience, same as cmd_users.go already does. This is not an
// engine change — UserStore.GetByEmail already existed.
func resolveUserID(db *sql.DB, email string) (string, error) {
	us := postgres.NewUserStore(db)
	user, err := us.GetByEmail(context.Background(), email)
	if err != nil {
		return "", fmt.Errorf("failed to find user %s: %w", email, err)
	}
	return user.ID, nil
}
