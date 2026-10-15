package main

import (
	"database/sql"
	"fmt"
	"os"
)

const csaxVersion = "dev" // set to the real tag (e.g. "v1.1.0") when this release is actually cut
const targetCrydenVersion = "cryden/v2 v2.5.0"

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

func cmdHealthAPI(cfg csaxConfig) {
	client, err := newAPIClient(cfg)
	if err != nil {
		fmt.Println(red("error: " + err.Error()))
		os.Exit(1)
	}
	// /v1/health is public — deliberately not routed through
	// client.get, which would try to log in first. A health check
	// should work even before an operator has ever authenticated,
	// the same way it works with no database credential in direct-DB
	// mode either.
	resp, err := client.http.Get(client.baseURL + "/v1/health")
	if err != nil {
		fmt.Println(red("✗ api unreachable: " + err.Error()))
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		fmt.Println(green("✔") + " api reachable at " + cfg.APIURL)
	} else {
		fmt.Println(red(fmt.Sprintf("✗ api answered %d — database is likely unreachable from the api side", resp.StatusCode)))
		os.Exit(1)
	}
}

func cmdVersion() {
	fmt.Printf("csax %s (%s)\n", csaxVersion, targetCrydenVersion)
}
