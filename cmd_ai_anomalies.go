package main

import (
	"context"
	"fmt"
	"os"

	"github.com/crydensync/cryden/v2/ai"
	"github.com/crydensync/cryden/v2/store/postgres"
)

// cmdAIAnomaliesScan is `ai logs` pre-run with a fixed, no-input
// prompt — deliberately implemented as a thin wrapper around the same
// auditOnlyProvider/ExecuteQuery path rather than a separate code
// path, so the two commands can never quietly drift apart in
// behavior.
func cmdAIAnomaliesScan(cfg csaxConfig, since string) {
	readonlyDB, provider := mustAISetup(cfg)
	defer readonlyDB.Close()

	store := postgres.NewSafeQueryStore(readonlyDB)
	auditProvider := &auditOnlyProvider{inner: provider}

	naturalLanguage := fmt.Sprintf(
		"unusual or suspicious activity in the last %s — repeated failed logins, token reuse detections, logins from new or unusual locations, or clusters of signups from the same source",
		since,
	)
	var result ai.QueryResult
	err := withSpinner("Scanning for anomalies...", func() error {
		var qerr error
		result, qerr = ai.ExecuteQuery(context.Background(), store, auditProvider, naturalLanguage)
		return qerr
	})
	if err != nil {
		fmt.Println(red("Anomaly scan failed: " + err.Error()))
		os.Exit(1)
	}

	if len(result.Rows) == 0 {
		fmt.Println(dim("No events found in that window."))
		return
	}

	var summary string
	summarizeErr := withSpinner("Summarizing...", func() error {
		var serr error
		summary, serr = provider.Summarize(context.Background(),
			"You are scanning audit log events for a system administrator, looking specifically for signs of "+
				"credential stuffing, account takeover attempts, or abuse. Some identifiers below are placeholders "+
				"(email_1, ip_2, etc.) standing in for real values — refer to them exactly as given, never invent a "+
				"real-looking email or IP. Summarize what you actually see in 2-4 plain-language sentences. Only "+
				"describe patterns present in the data given — never invent detail. If nothing looks concerning, "+
				"say so plainly rather than manufacturing a finding.",
			redactForSummary(result))
		return serr
	})
	if summarizeErr != nil {
		fmt.Println(yellow("(could not generate a summary: " + summarizeErr.Error() + ")"))
	} else {
		fmt.Println(summary)
		fmt.Println()
	}

	fmt.Println(dim("Run `csax ai logs \"...\"` for detail on any of these, or `csax ai query` for user-level data."))
	fmt.Println()
	printQueryResult(result, false)
}
