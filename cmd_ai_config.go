package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/crydensync/cryden/v2/ai"
)

// cmdAIConfig interactively sets up everything `csax ai query`/`ai
// logs`/`ai audit` need: the AI provider, model, API key env var
// name, and — the part that caused the most friction in manual
// testing — a genuinely read-only Postgres role, created for the
// user rather than handed to them as a script to run by hand.
func cmdAIConfig(cfg csaxConfig) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Setting up AI-assisted commands (ai query, ai logs, ai audit).")
	fmt.Println()

	provider := promptDefault(reader, "AI provider (groq/openrouter)", "groq")
	model := promptDefault(reader, "Model id", defaultModelFor(provider))
	apiKeyEnv := promptDefault(reader, "Env var name holding your API key (the key itself is never stored here)", strings.ToUpper(provider)+"_API_KEY")

	fmt.Println()
	values := map[string]string{
		"AI_PROVIDER":    provider,
		"AI_MODEL":       model,
		"AI_API_KEY_ENV": apiKeyEnv,
	}

	if promptYesNo(reader, "Set up a read-only database role now? (recommended — required for ai query/ai logs)", true) {
		readonlyURL := setupReadonlyRole(reader, cfg)
		if readonlyURL != "" {
			values["READONLY_DATABASE_URL"] = readonlyURL
		}
	} else {
		fmt.Println("Skipped. Run `csax ai config` again later, or set READONLY_DATABASE_URL by hand — see the README for the manual SQL.")
	}

	appendEnvValues(reader, values)
	fmt.Println()
	fmt.Printf("Don't forget: export %s=<your actual key> before running ai commands.\n", apiKeyEnv)
}

func defaultModelFor(provider string) string {
	switch provider {
	case "groq":
		return "openai/gpt-oss-20b"
	case "openrouter":
		return "nvidia/nemotron-nano-9b-v2:free"
	default:
		return ""
	}
}

// setupReadonlyRole creates a Postgres role scoped to exactly the
// tables ai.AllowedEntities covers — the same allowlist ExecuteQuery
// enforces, so this list can never silently drift from what's
// actually queryable. Runs the SQL directly via the admin
// DATABASE_URL already in cfg, rather than handing the user a script
// to run by hand and hoping they get the connection right.
func setupReadonlyRole(reader *bufio.Reader, cfg csaxConfig) string {
	roleName := promptDefault(reader, "Read-only role name", "csax_readonly")
	password := promptDefault(reader, "Password for this role (leave blank to auto-generate)", "")
	generated := password == ""
	if generated {
		password = generateSecret()[:24]
	}

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		fmt.Printf("could not open admin DB connection: %v\n", err)
		return ""
	}
	defer db.Close()

	dbName, err := currentDatabaseName(db)
	if err != nil {
		fmt.Printf("could not determine current database name: %v\n", err)
		return ""
	}

	// Tables are taken from ai.AllowedEntities, not hardcoded here —
	// if that allowlist ever changes, this wizard's grants
	// automatically follow it instead of silently going stale.
	tables := make([]string, 0, len(ai.AllowedEntities))
	for name := range ai.AllowedEntities {
		tables = append(tables, name)
	}
	sort.Strings(tables)

	statements := []string{
		fmt.Sprintf("CREATE ROLE %s WITH LOGIN PASSWORD %s", pqIdent(roleName), pqLiteral(password)),
		fmt.Sprintf("GRANT CONNECT ON DATABASE %s TO %s", pqIdent(dbName), pqIdent(roleName)),
		fmt.Sprintf("GRANT USAGE ON SCHEMA public TO %s", pqIdent(roleName)),
	}
	for _, t := range tables {
		statements = append(statements, fmt.Sprintf("GRANT SELECT ON %s TO %s", pqIdent(t), pqIdent(roleName)))
	}

	fmt.Printf("Creating role %q with SELECT on: %s\n", roleName, strings.Join(tables, ", "))
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			// A role that already exists is a common, harmless case
			// (re-running this wizard) — don't abort the whole setup
			// over it, just note it and keep going with the grants.
			if strings.Contains(err.Error(), "already exists") {
				fmt.Printf("  (skipped, already exists: %s)\n", firstWords(stmt, 4))
				continue
			}
			fmt.Printf("failed running: %s\n  error: %v\n", stmt, err)
			return ""
		}
	}
	fmt.Println("✔ Role created and granted.")

	readonlyURL, err := buildReadonlyURL(cfg.DatabaseURL, roleName, password)
	if err != nil {
		fmt.Printf("role was created, but couldn't build the connection string automatically: %v\n", err)
		fmt.Println("Build it by hand: same host/port/database as DATABASE_URL, with this role's credentials.")
		return ""
	}

	testDB, err := sql.Open("postgres", readonlyURL)
	if err == nil {
		defer testDB.Close()
		var pingErr error
		withSpinner("Verifying connection...", func() error {
			pingErr = testDB.Ping()
			return nil // spinner just reports timing here, not success/failure
		})
		if pingErr == nil {
			fmt.Println("✔ Verified: the read-only connection works.")
		} else {
			fmt.Printf("⚠ Role created, but the test connection failed: %v\n", pingErr)
			fmt.Println("  This can happen with some poolers (e.g. Supabase) needing a moment to recognize a new role — try `csax ai query` again shortly.")
		}
	}

	if generated {
		fmt.Printf("Generated password: %s — this is only shown once, save it if you need it separately.\n", password)
	}

	return readonlyURL
}

// buildReadonlyURL constructs the read-only connection string from
// the existing admin DATABASE_URL, swapping in the new role's
// credentials. Handles ONE known provider-specific quirk explicitly
// (Supabase's pooler requires <role>.<project-ref> as the username)
// rather than assuming every deployment works that way — a plain
// self-hosted Postgres, Neon, RDS, etc. all just get the plain role
// name.
func buildReadonlyURL(adminURL, roleName, password string) (string, error) {
	u, err := url.Parse(adminURL)
	if err != nil {
		return "", err
	}

	newUsername := roleName
	if isSupabasePoolerHost(u.Hostname()) {
		if existing := u.User.Username(); existing != "" {
			if dot := strings.LastIndex(existing, "."); dot != -1 {
				projectRef := existing[dot+1:]
				newUsername = roleName + "." + projectRef
			}
		}
	}

	u.User = url.UserPassword(newUsername, password)
	return u.String(), nil
}

func isSupabasePoolerHost(host string) bool {
	return strings.Contains(host, "supabase.com") || strings.Contains(host, "supabase.co")
}

func currentDatabaseName(db *sql.DB) (string, error) {
	var name string
	err := db.QueryRow("SELECT current_database()").Scan(&name)
	return name, err
}

// pqIdent and pqLiteral do minimal, defensive escaping for values
// interpolated into DDL statements that Postgres' protocol can't
// parameterize (role/table names, CREATE ROLE's password clause).
// This is wizard input the person is typing about their own
// database, not attacker-controlled — but escaping costs nothing and
// avoids a broken role name silently becoming a SQL syntax error or
// worse.
func pqIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func pqLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

func firstWords(s string, n int) string {
	words := strings.Fields(s)
	if len(words) > n {
		words = words[:n]
	}
	return strings.Join(words, " ") + "..."
}

func promptDefault(reader *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func promptYesNo(reader *bufio.Reader, label string, def bool) bool {
	suffix := "[Y/n]"
	if !def {
		suffix = "[y/N]"
	}
	fmt.Printf("%s %s: ", label, suffix)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return def
	}
	return line == "y" || line == "yes"
}

// appendEnvValues writes/updates .env with the given key-value pairs,
// preserving whatever's already there — cmd_config.go's writer
// replaces the whole file, which would destroy DATABASE_URL/JWT_SECRET
// if reused here, so this reads first and only touches matching keys.
func appendEnvValues(reader *bufio.Reader, values map[string]string) {
	existing := map[string]string{}
	var order []string
	if data, err := os.ReadFile(".env"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if eq := strings.Index(line, "="); eq != -1 {
				key := line[:eq]
				existing[key] = line[eq+1:]
				order = append(order, key)
			}
		}
	}
	for k, v := range values {
		if _, ok := existing[k]; !ok {
			order = append(order, k)
		}
		existing[k] = v
	}

	var b strings.Builder
	for _, k := range order {
		fmt.Fprintf(&b, "%s=%s\n", k, existing[k])
	}
	if err := os.WriteFile(".env", []byte(b.String()), 0600); err != nil {
		fmt.Printf("failed to write .env: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✔ Updated .env")
}
