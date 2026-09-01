package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/crydensync/cryden/v2/ai"
	"github.com/crydensync/cryden/v2/store/postgres"
)

// cmdAIAsk implements `csax ai ask "<question>"` (aliased as `csax
// audit ask`). Unlike `ai logs`, this does NOT force Entity to
// audit_events — a real operator question ("anything weird with
// devray@example.com this week?") may need to look at users or
// sessions too, not just the audit trail. Same ExecuteQuery/
// SafeQueryStore path as `ai query`, phrased as a direct question
// with a narrative answer instead of a raw table.
func cmdAIAsk(cfg csaxConfig, question string) {
	readonlyDB, provider := mustAISetup(cfg)
	defer readonlyDB.Close()

	store := postgres.NewSafeQueryStore(readonlyDB)
	var result ai.QueryResult
	err := withSpinner("Thinking...", func() error {
		var qerr error
		result, qerr = ai.ExecuteQuery(context.Background(), store, provider, question)
		return qerr
	})
	if err != nil {
		if errors.Is(err, ai.ErrUnsafeQueryIntent) {
			fmt.Println(red("That question can't be translated into an allowed query."))
			fmt.Println(dim("Try rephrasing, or ask about one of: users, sessions, audit_events."))
			os.Exit(1)
		}
		fmt.Println(red("Couldn't answer that: " + err.Error()))
		os.Exit(1)
	}

	if len(result.Rows) == 0 {
		fmt.Println(dim("No matching data found."))
		return
	}

	var answer string
	answerErr := withSpinner("Summarizing...", func() error {
		var aerr error
		answer, aerr = provider.Summarize(context.Background(),
			"You are answering a system administrator's direct question about their own application's data. "+
				"Some identifiers below are placeholders (email_1, ip_2, etc.) standing in for real values — refer "+
				"to them exactly as given, never invent a real-looking email or IP. Answer the question directly in "+
				"2-4 plain-language sentences, using only what's in the data given. If the data doesn't actually "+
				"answer the question, say so rather than guessing.",
			question+"\n\nData:\n"+redactForSummary(result))
		return aerr
	})
	if answerErr != nil {
		fmt.Println(yellow("(could not generate an answer: " + answerErr.Error() + ")"))
	} else {
		fmt.Println(answer)
		fmt.Println()
	}

	printQueryResult(result, false)
}
