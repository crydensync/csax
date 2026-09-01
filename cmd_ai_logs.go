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
	// originalEntity records what the model actually chose, before
	// being overridden below — lets cmdAILogs warn the person when
	// they've asked a question this command was never going to
	// answer (e.g. "show me all users" silently returning audit
	// events instead, with no indication that's not what they meant).
	originalEntity string
}

func (p *auditOnlyProvider) ParseQueryIntent(ctx context.Context, naturalLanguage string) (ai.QueryIntent, error) {
	intent, err := p.inner.ParseQueryIntent(ctx, naturalLanguage)
	if err != nil {
		return ai.QueryIntent{}, err
	}
	p.originalEntity = intent.Entity
	intent.Entity = "audit_events" // this command is audit-only by definition — never trust the model's entity choice here
	// Force real rows, never an aggregate — `ai logs` exists to show
	// actual events for review. Left to the model, a vague prompt
	// like "any failed logins recently" can get interpreted as
	// Aggregate: "count", which produces a single useless number
	// instead of the log entries the command is actually for.
	intent.Aggregate = ""
	intent.GroupBy = ""
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
	auditProvider := &auditOnlyProvider{inner: provider}
	var result ai.QueryResult
	err := withSpinner("Searching audit log...", func() error {
		var qerr error
		result, qerr = ai.ExecuteQuery(context.Background(), store, auditProvider, naturalLanguage)
		return qerr
	})
	if err != nil {
		fmt.Println(red("Log search failed: " + err.Error()))
		os.Exit(1)
	}

	if auditProvider.originalEntity != "" && auditProvider.originalEntity != "audit_events" {
		fmt.Println(yellow(fmt.Sprintf("(Note: ai logs only searches audit events — your question looked like it was about %s. Try `csax ai query` for that.)", auditProvider.originalEntity)))
		fmt.Println()
	}

	if len(result.Rows) == 0 {
		fmt.Println(dim("No matching audit events."))
		return
	}

	var summary string
	summarizeErr := withSpinner("Summarizing...", func() error {
		var serr error
		summary, serr = provider.Summarize(context.Background(),
			"You summarize a list of audit log events for a system administrator in 2-3 plain-language sentences. "+
				"Some identifiers below are placeholders (email_1, ip_2, etc.) standing in for real values — refer to "+
				"them exactly as given, never invent a real-looking email or IP. Only describe patterns you can see "+
				"in the data given. Never suggest the administrator take an action you're not certain about; "+
				"if a corrective action seems relevant, phrase it as a suggestion, never as something already done.",
			redactForSummary(result))
		return serr
	})
	if summarizeErr != nil {
		fmt.Println(yellow("(could not generate a summary: " + summarizeErr.Error() + ")"))
	} else {
		fmt.Println(summary)
		fmt.Println()
	}

	printQueryResult(result, false)
}
