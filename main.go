package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/crydensync/cryden/v2"
)

const logo = `
   ▄████████▄ ▄▄▄
   █ ██████▀▘ ▄▄▄▄▄  csax — CrydenSync admin CLI
   █ ▀▀▀▀▘   ▀▀▀▀▀▘  self-hosted auth, owned by you
`

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
  csax audit ask "<question>"
  csax oauth providers list [--json] | add <provider>
  csax oauth test <provider>
  csax oauth users get <email> [--json]
  csax oauth unlink <email> --provider <name>
  csax oauth config
  csax ai query "<natural language>" [--json]
  csax ai logs "<natural language>"
  csax ai anomalies scan [--since 24h]
  csax ai ask "<question>"
  csax ai audit
  csax ai config
  csax doctor
  csax stats
  csax health
  csax version

oauth and ai commands are optional — see README for the env vars each
one needs, or run ` + "`csax ai config`" + ` / ` + "`csax oauth config`" + ` for an
interactive setup. Run any command with no further args for its
specific usage.`)
}

func main() {
	if len(os.Args) < 2 {
		if colorsEnabled {
			fmt.Println(dim(logo))
		}
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
			fmt.Println(`usage: csax audit tail|search [args] | ask "<question>"`)
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
		case "ask":
			// alias for `ai ask` — same underlying command, just
			// phrased the way someone thinking "ask about my audit
			// trail" is more likely to type it. db was already
			// opened above for tail/search but isn't needed here —
			// cmdAIAsk opens its own read-only connection.
			if len(os.Args) < 4 {
				fmt.Println(`usage: csax audit ask "<question>"`)
				os.Exit(1)
			}
			cmdAIAsk(cfg, os.Args[3])
		default:
			fmt.Println(`usage: csax audit tail|search [args] | ask "<question>"`)
			os.Exit(1)
		}

	case "oauth":
		cfg := mustLoadConfig()
		if len(os.Args) < 3 {
			fmt.Println("usage: csax oauth providers list | test <provider> | users get <email> | unlink <email> --provider <name>")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "providers":
			if len(os.Args) < 4 {
				fmt.Println("usage: csax oauth providers list [--json] | add <provider>")
				os.Exit(1)
			}
			switch os.Args[3] {
			case "list":
				fs := flag.NewFlagSet("oauth providers list", flag.ExitOnError)
				jsonOut := fs.Bool("json", false, "output as JSON")
				fs.Parse(os.Args[4:])
				cmdOAuthProvidersList(cfg, *jsonOut)
			case "add":
				if len(os.Args) < 5 {
					fmt.Println("usage: csax oauth providers add <provider>")
					os.Exit(1)
				}
				cmdOAuthProvidersAdd(cfg, os.Args[4])
			default:
				fmt.Println("usage: csax oauth providers list [--json] | add <provider>")
				os.Exit(1)
			}
		case "test":
			if len(os.Args) < 4 {
				fmt.Println("usage: csax oauth test <provider>")
				os.Exit(1)
			}
			cmdOAuthTest(cfg, os.Args[3])
		case "users":
			if len(os.Args) < 5 || os.Args[3] != "get" {
				fmt.Println("usage: csax oauth users get <email> [--json]")
				os.Exit(1)
			}
			db := mustConnect(cfg)
			defer db.Close()
			fs := flag.NewFlagSet("oauth users get", flag.ExitOnError)
			jsonOut := fs.Bool("json", false, "output as JSON")
			fs.Parse(os.Args[5:])
			cmdOAuthUsersGet(db, os.Args[4], *jsonOut)
		case "unlink":
			if len(os.Args) < 4 {
				fmt.Println("usage: csax oauth unlink <email> --provider <name>")
				os.Exit(1)
			}
			fs := flag.NewFlagSet("oauth unlink", flag.ExitOnError)
			providerFlag := fs.String("provider", "", "provider to unlink (required)")
			fs.Parse(os.Args[4:])
			db := mustConnect(cfg)
			defer db.Close()
			cmdOAuthUnlink(db, os.Args[3], *providerFlag)
		case "config":
			cmdOAuthConfig(cfg)
		default:
			fmt.Println("usage: csax oauth providers list | test <provider> | users get <email> | unlink <email> --provider <name> | config")
			os.Exit(1)
		}

	case "ai":
		cfg := mustLoadConfig()
		if len(os.Args) < 3 {
			fmt.Println(`usage: csax ai query "<...>" | logs "<...>" | audit | config | anomalies scan [--since 24h] | ask "<...>"`)
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
		case "config":
			cmdAIConfig(cfg)
		case "anomalies":
			if len(os.Args) < 4 || os.Args[3] != "scan" {
				fmt.Println("usage: csax ai anomalies scan [--since 24h]")
				os.Exit(1)
			}
			fs := flag.NewFlagSet("ai anomalies scan", flag.ExitOnError)
			since := fs.String("since", "24h", "how far back to look")
			fs.Parse(os.Args[4:])
			cmdAIAnomaliesScan(cfg, *since)
		case "ask":
			if len(os.Args) < 4 {
				fmt.Println(`usage: csax ai ask "<question>"`)
				os.Exit(1)
			}
			cmdAIAsk(cfg, os.Args[3])
		default:
			fmt.Println(`usage: csax ai query "<...>" | logs "<...>" | audit | config | anomalies scan [--since 24h] | ask "<...>"`)
			os.Exit(1)
		}

	// `audit ask` is an alias for `ai ask`, handled inside the real
	// top-level `case "audit":` block above (not duplicated here —
	// Go doesn't allow two cases with the same value in one switch).

	case "doctor":
		cfg := mustLoadConfig()
		db := mustConnect(cfg)
		defer db.Close()
		cmdDoctor(cfg, db)

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
