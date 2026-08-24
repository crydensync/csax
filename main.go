package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/crydensync/cryden/v2"
)

func usage() {
	fmt.Println(`csax — CrydenSync admin CLI

Usage:
  csax config init
  csax migrate up|down|status
  csax users list [--limit N] [--offset N] [--json]
  csax users get <email> [--json]
  csax users create <email> <password>
  csax users unlock <email>
  csax sessions list --user <email> [--json]
  csax sessions revoke <session-id> --user <email>
  csax sessions revoke-all --user <email>
  csax audit tail --user <email> [--limit N]
  csax audit search --event <type> [--limit N]
  csax oauth providers list [--json]
  csax oauth test <provider>
  csax ai query "<natural language>" [--json]
  csax ai logs "<natural language>"
  csax ai audit
  csax stats
  csax health
  csax version

oauth and ai commands are optional — see README for the env vars each
one needs. Run any command with no further args for its specific usage.`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "config":
		if len(os.Args) < 3 || os.Args[2] != "init" {
			fmt.Println("usage: csax config init")
			os.Exit(1)
		}
		cmdConfigInit(os.Args[3:])

	case "migrate":
		cfg := mustLoadConfig()
		db := mustConnect(cfg)
		defer db.Close()
		if len(os.Args) < 3 {
			fmt.Println("usage: csax migrate up|down|status")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "up":
			cmdMigrateUp(cfg, db)
		case "down":
			cmdMigrateDown(cfg, db)
		case "status":
			cmdMigrateStatus(cfg, db)
		default:
			fmt.Println("usage: csax migrate up|down|status")
			os.Exit(1)
		}

	case "users":
		cfg := mustLoadConfig()
		db := mustConnect(cfg)
		defer db.Close()
		if len(os.Args) < 3 {
			fmt.Println("usage: csax users list|get|create|unlock [args]")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "list":
			fs := flag.NewFlagSet("users list", flag.ExitOnError)
			limit := fs.Int("limit", 50, "max users to show")
			offset := fs.Int("offset", 0, "pagination offset")
			jsonOut := fs.Bool("json", false, "output as JSON")
			fs.Parse(os.Args[3:])
			cmdUsersList(db, *limit, *offset, *jsonOut)
		case "get":
			if len(os.Args) < 4 {
				fmt.Println("usage: csax users get <email> [--json]")
				os.Exit(1)
			}
			fs := flag.NewFlagSet("users get", flag.ExitOnError)
			jsonOut := fs.Bool("json", false, "output as JSON")
			fs.Parse(os.Args[4:])
			cmdUsersGet(db, os.Args[3], *jsonOut)
		case "create":
			if len(os.Args) < 5 {
				fmt.Println("usage: csax users create <email> <password>")
				os.Exit(1)
			}
			engine := mustBuildEngine(cfg, db)
			cmdUsersCreate(engine, os.Args[3], os.Args[4])
		case "unlock":
			if len(os.Args) < 4 {
				fmt.Println("usage: csax users unlock <email>")
				os.Exit(1)
			}
			cmdUsersUnlock(db, os.Args[3])
		default:
			fmt.Println("usage: csax users list|get|create|unlock [args]")
			os.Exit(1)
		}

	case "sessions":
		cfg := mustLoadConfig()
		db := mustConnect(cfg)
		defer db.Close()
		if len(os.Args) < 3 {
			fmt.Println("usage: csax sessions list|revoke|revoke-all")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "list":
			fs := flag.NewFlagSet("sessions list", flag.ExitOnError)
			user := fs.String("user", "", "user email")
			jsonOut := fs.Bool("json", false, "output as JSON")
			fs.Parse(os.Args[3:])
			cmdSessionsList(db, *user, *jsonOut)

		case "revoke":
			fs := flag.NewFlagSet("sessions revoke", flag.ExitOnError)
			user := fs.String("user", "", "user email")
			if len(os.Args) < 4 {
				fmt.Println("usage: csax sessions revoke <session-id> --user <email>")
				os.Exit(1)
			}
			sessionID := os.Args[3]
			fs.Parse(os.Args[4:])
			engine := mustBuildEngine(cfg, db)
			cmdSessionsRevoke(db, engine, *user, sessionID)

		case "revoke-all":
			fs := flag.NewFlagSet("sessions revoke-all", flag.ExitOnError)
			user := fs.String("user", "", "user email")
			fs.Parse(os.Args[3:])
			engine := mustBuildEngine(cfg, db)
			cmdSessionsRevokeAll(db, engine, *user)

		default:
			fmt.Println("usage: csax sessions list|revoke|revoke-all")
			os.Exit(1)
		}

	case "audit":
		cfg := mustLoadConfig()
		db := mustConnect(cfg)
		defer db.Close()
		if len(os.Args) < 3 {
			fmt.Println("usage: csax audit tail|search [args]")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "tail":
			fs := flag.NewFlagSet("audit tail", flag.ExitOnError)
			user := fs.String("user", "", "user email")
			limit := fs.Int("limit", 20, "max events to show")
			fs.Parse(os.Args[3:])
			cmdAuditTail(db, *user, *limit)
		case "search":
			fs := flag.NewFlagSet("audit search", flag.ExitOnError)
			event := fs.String("event", "", "event type, e.g. login_failed, token_reuse_detected")
			limit := fs.Int("limit", 20, "max events to show")
			fs.Parse(os.Args[3:])
			cmdAuditSearch(db, *event, *limit)
		default:
			fmt.Println("usage: csax audit tail|search [args]")
			os.Exit(1)
		}

	case "oauth":
		cfg := mustLoadConfig()
		if len(os.Args) < 3 {
			fmt.Println("usage: csax oauth providers list | test <provider>")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "providers":
			if len(os.Args) < 4 || os.Args[3] != "list" {
				fmt.Println("usage: csax oauth providers list [--json]")
				os.Exit(1)
			}
			fs := flag.NewFlagSet("oauth providers list", flag.ExitOnError)
			jsonOut := fs.Bool("json", false, "output as JSON")
			fs.Parse(os.Args[4:])
			cmdOAuthProvidersList(cfg, *jsonOut)
		case "test":
			if len(os.Args) < 4 {
				fmt.Println("usage: csax oauth test <provider>")
				os.Exit(1)
			}
			cmdOAuthTest(cfg, os.Args[3])
		default:
			fmt.Println("usage: csax oauth providers list | test <provider>")
			os.Exit(1)
		}

	case "ai":
		cfg := mustLoadConfig()
		if len(os.Args) < 3 {
			fmt.Println(`usage: csax ai query "<natural language>" | logs "<natural language>" | audit`)
			os.Exit(1)
		}
		switch os.Args[2] {
		case "query":
			if len(os.Args) < 4 {
				fmt.Println(`usage: csax ai query "<natural language>" [--json]`)
				os.Exit(1)
			}
			fs := flag.NewFlagSet("ai query", flag.ExitOnError)
			jsonOut := fs.Bool("json", false, "output as JSON")
			fs.Parse(os.Args[4:])
			cmdAIQuery(cfg, os.Args[3], *jsonOut)
		case "logs":
			if len(os.Args) < 4 {
				fmt.Println(`usage: csax ai logs "<natural language>"`)
				os.Exit(1)
			}
			cmdAILogs(cfg, os.Args[3])
		case "audit":
			cmdAIAudit(cfg)
		default:
			fmt.Println(`usage: csax ai query "<natural language>" | logs "<natural language>" | audit`)
			os.Exit(1)
		}

	case "stats":
		cfg := mustLoadConfig()
		db := mustConnect(cfg)
		defer db.Close()
		cmdStats(db)

	case "health":
		cfg := mustLoadConfig()
		db := mustConnect(cfg)
		defer db.Close()
		cmdHealth(db)

	case "version":
		cmdVersion()

	default:
		usage()
		os.Exit(1)
	}
}

func mustLoadConfig() csaxConfig {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return cfg
}

func mustConnect(cfg csaxConfig) *sql.DB {
	db, err := connectDB(cfg)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return db
}

func mustBuildEngine(cfg csaxConfig, db *sql.DB) *cryden.Engine {
	engine, err := buildEngine(cfg, db)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return engine
}
