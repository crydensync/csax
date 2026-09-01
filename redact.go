package main

import (
	"fmt"
	"strings"

	"github.com/crydensync/cryden/v2/ai"
)

// redactForSummary builds the text sent to an AI provider for
// summarization, replacing email and IP values with stable
// per-result placeholders (email_1, ip_2, ...) rather than sending
// real PII to a third-party provider.
//
// This is a deliberate default, not a policy someone else already
// signed off on — flagging that plainly. Unlike the docs site's "Ask
// AI" (which only ever sends public documentation text), audit data
// contains real emails and IPs, and CrydenSync's own operator may not
// want that leaving their infrastructure even to a provider they
// trust for other things. Placeholders are stable WITHIN one result
// set, so repetition and clustering (e.g. "email_3 appears 5 times")
// are still visible to the model — the actual values just never are.
//
// The real, unredacted data is still what gets PRINTED to the
// terminal afterward (see printQueryResult in each caller) — only the
// text handed to Summarize goes through this.
func redactForSummary(result ai.QueryResult) string {
	emailIdx, ipIdx := -1, -1
	for i, col := range result.Columns {
		switch col {
		case "email":
			emailIdx = i
		case "ip":
			ipIdx = i
		}
	}

	emails := map[string]string{}
	ips := map[string]string{}

	var b strings.Builder
	for _, row := range result.Rows {
		for i, v := range row {
			col := result.Columns[i]
			switch i {
			case emailIdx:
				v = pseudonym(emails, v, "email")
			case ipIdx:
				v = pseudonym(ips, v, "ip")
			}
			fmt.Fprintf(&b, "%s=%s ", col, v)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func pseudonym(seen map[string]string, value, label string) string {
	if value == "" {
		return value
	}
	if existing, ok := seen[value]; ok {
		return existing
	}
	placeholder := fmt.Sprintf("%s_%d", label, len(seen)+1)
	seen[value] = placeholder
	return placeholder
}
