package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/crydensync/cryden/v2/ai"
	"github.com/crydensync/cryden/v2/store/postgres"
)

// cmdAIQuery implements `csax ai query "<natural language>"`. Three
// distinct failure messages on purpose — an unsafe intent, a provider
// failure, and a DB failure are different problems for the operator,
// and conflating them into one generic "query failed" would make this
// harder to debug than just writing raw SQL would have been.
func cmdAIQuery(cfg csaxConfig, naturalLanguage string, jsonOutput bool) {
	readonlyDB, provider := mustAISetup(cfg)
	defer readonlyDB.Close()

	store := postgres.NewSafeQueryStore(readonlyDB)
	result, err := ai.ExecuteQuery(context.Background(), store, provider, naturalLanguage)
	if err != nil {
		if errors.Is(err, ai.ErrUnsafeQueryIntent) {
			// Deliberately does NOT echo the model's raw output or
			// the specific rejected field/entity back to the
			// terminal — that's an easy vector for confusing or
			// misleading text to end up in a script's output.
			fmt.Println(red("That request can't be translated into an allowed query."))
			fmt.Println(dim("Try rephrasing, or ask about one of: users, sessions, audit_events."))
			os.Exit(1)
		}
		fmt.Println(red("Query failed: " + err.Error()))
		os.Exit(1)
	}

	printQueryResult(result, jsonOutput)
}

func printQueryResult(result ai.QueryResult, jsonOutput bool) {
	if jsonOutput {
		out := map[string]any{"columns": result.Columns, "rows": result.Rows}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return
	}

	if len(result.Rows) == 0 {
		fmt.Println(dim("No rows."))
		return
	}
	for _, col := range result.Columns {
		fmt.Printf("%-24s", col)
	}
	fmt.Println()
	for _, row := range result.Rows {
		for _, v := range row {
			fmt.Printf("%-24s", v)
		}
		fmt.Println()
	}
	fmt.Printf("\n%d row(s).\n", len(result.Rows))
}

// mustAISetup builds the two things every ai command needs: a real
// LLMProvider and a read-only-role Postgres connection for
// SafeQueryStore. Exits with a clear message if either isn't
// configured — ai commands are the one part of csax that's entirely
// optional to set up, so a missing config here should never look like
// a crash.
func mustAISetup(cfg csaxConfig) (*sql.DB, *llmProvider) {
	if cfg.ReadOnlyDBURL == "" {
		fmt.Println(red("READONLY_DATABASE_URL is not set."))
		fmt.Println(dim("AI-assisted queries require a SEPARATE connection string pointing at a read-only Postgres role — this is the real safety boundary, not just the allowlist check. See the docs for setting one up."))
		os.Exit(1)
	}
	provider, err := newLLMProvider(cfg)
	if err != nil {
		fmt.Println(red("AI is not configured: " + err.Error()))
		os.Exit(1)
	}

	db, err := sql.Open("postgres", cfg.ReadOnlyDBURL)
	if err != nil {
		fmt.Println(red("could not open read-only DB connection: " + err.Error()))
		os.Exit(1)
	}
	if err := db.Ping(); err != nil {
		fmt.Println(red("could not reach read-only DB: " + err.Error()))
		os.Exit(1)
	}
	return db, provider
}
