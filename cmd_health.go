package main

import (
	"database/sql"
	"fmt"
)

const csaxVersion = "dev" // set to the real tag (e.g. "v1.1.0") when this release is actually cut
const targetCrydenVersion = "cryden/v2 v2.1.0"

func cmdHealth(db *sql.DB) {
	if err := db.Ping(); err != nil {
		fmt.Printf("✗ Database unreachable: %v\n", err)
		return
	}
	fmt.Println("✔ Database reachable")

	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'csax_migrations'`).Scan(&count)
	if err != nil || count == 0 {
		fmt.Println("✗ Migrations not yet tracked — run `csax migrate up`")
		return
	}
	fmt.Println("✔ Migration tracking present")
}

func cmdVersion() {
	fmt.Printf("csax %s (%s)\n", csaxVersion, targetCrydenVersion)
}
