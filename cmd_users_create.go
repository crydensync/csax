package main

import (
	"context"
	"fmt"
	"os"

	"github.com/crydensync/cryden/v2"
)

func cmdUsersCreate(engine *cryden.Engine, email, password string) {
	if email == "" || password == "" {
		fmt.Println("usage: csax users create <email> <password>")
		os.Exit(1)
	}
	ctx := context.Background()

	// callerIP is required by SignUp's signature since the engine
	// never infers it — "cli" is used here as an honest label, since
	// there's no real network caller IP for a local admin command.
	user, err := cryden.SignUp(ctx, engine, email, password, "cli")
	if err != nil {
		fmt.Println(red("✗ " + err.Error()))
		os.Exit(1)
	}
	fmt.Println(green("✔ Created user"))
	fmt.Printf("ID:    %s\n", user.ID)
	fmt.Printf("Email: %s\n", user.Email)
}
