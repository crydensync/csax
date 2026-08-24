package main

import (
	"context"
	"fmt"
	"os"

	"github.com/crydensync/cryden/v2/ai"
	"github.com/crydensync/cryden/v2/store/postgres"
)

// auditOnlyProvider wraps llmProvider and forces the parsed
// intent's Entity to "audit_events" regardless of what the model
// returned. Resolves the open design question from the earlier
// design doc: `ai logs` reuses the same QueryIntent/ExecuteQuery
// machinery as `ai query` rather than a separate narrower type, since
// the entity restriction is simpler to enforce this way — one
// validated path, not two.
type auditOnlyProvider struct {
	inner *llmProvider
}

func (p auditOnlyProvider) ParseQueryIntent(ctx context.Context, naturalLanguage string) (ai.QueryIntent, error) {
	intent, err := p.inner.ParseQueryIntent(ctx, naturalLanguage)
	if err != nil {
		return ai.QueryIntent{}, err
	}
	intent.Entity = "audit_events" // this command is audit-only by definition — never trust the model's entity choice here
	return intent, nil
}

// cmdAILogs implements `csax ai logs "<natural language>"`. It only
// ever reads via the same read-only SafeQueryStore as `ai query` and
// summarizes — nothing here suggests an action that gets executed
// automatically. If the summary mentions revoking a session or
// locking an account, that's plain text the operator acts on
// themselves with a separate, explicit command.
func cmdAILogs(cfg csaxConfig, naturalLanguage string) {
	readonlyDB, provider := mustAISetup(cfg)
	defer readonlyDB.Close()

	store := postgres.NewSafeQueryStore(readonlyDB)
	result, err := ai.ExecuteQuery(context.Background(), store, auditOnlyProvider{inner: provider}, naturalLanguage)
	if err != nil {
		fmt.Println(red("Log search failed: " + err.Error()))
		os.Exit(1)
	}

	if len(result.Rows) == 0 {
		fmt.Println(dim("No matching audit events."))
		return
	}

	summary, err := provider.Summarize(context.Background(),
		"You summarize a list of audit log events for a system administrator in 2-3 plain-language sentences. "+
			"Only describe patterns you can see in the data given. Never suggest the administrator take an action you're not certain about; "+
			"if a corrective action seems relevant, phrase it as a suggestion, never as something already done.",
		formatRowsForSummary(result))
	if err != nil {
		fmt.Println(yellow("(could not generate a summary: " + err.Error() + ")"))
	} else {
		fmt.Println(summary)
		fmt.Println()
	}

	printQueryResult(result, false)
}

func formatRowsForSummary(result ai.QueryResult) string {
	out := ""
	for _, row := range result.Rows {
		for i, v := range row {
			out += result.Columns[i] + "=" + v + " "
		}
		out += "\n"
	}
	return out
}
