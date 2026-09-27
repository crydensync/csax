package main

// API-client mode only, and permanently so — unlike digest/support/
// config-tuning, this isn't a matter of not having gotten to a
// direct-DB path yet. The review-status table this reads
// (reviewed_anomalies) is api's own addition to its own Postgres
// schema, not part of cryden's schema at all. A direct-DB
// implementation here would mean csax reaching into a table that
// belongs to a different repository's migrations — exactly the kind
// of cross-repository coupling this whole project has avoided
// everywhere else. See CLAUDE.md.

import (
	"encoding/json"
	"fmt"
	"os"
)

func cmdAnomaliesList(cfg csaxConfig, status string, limit int, jsonOutput bool) {
	if !apiMode(cfg) {
		fmt.Println("`csax anomalies` requires API-client mode — set CSAX_API_URL. The review queue is api's own table, not part of cryden's schema, so there is no direct-DB path for this command at all.")
		os.Exit(1)
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		fmt.Println(red("error: " + err.Error()))
		os.Exit(1)
	}

	var entries []struct {
		EventID   string `json:"event_id"`
		Type      string `json:"type"`
		UserID    string `json:"user_id"`
		Status    string `json:"status"`
		Note      string `json:"note,omitempty"`
		CreatedAt string `json:"created_at"`
	}
	path := fmt.Sprintf("/admin/anomalies?limit=%d", limit)
	if status != "" {
		path += "&status=" + status
	}
	if err := client.get(cfg, path, &entries); err != nil {
		fmt.Println(red("request failed: " + err.Error()))
		os.Exit(1)
	}

	if jsonOutput {
		b, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(b))
		return
	}
	if len(entries) == 0 {
		fmt.Println(dim("No flagged events found for this filter."))
		return
	}
	columns := []string{"EVENT ID", "TYPE", "USER", "STATUS", "CREATED"}
	var rows [][]string
	for _, e := range entries {
		rows = append(rows, []string{e.EventID, e.Type, e.UserID, anomalyStatusColor(e.Status), e.CreatedAt})
	}
	printTable(columns, rows)
}

func anomalyStatusColor(status string) string {
	switch status {
	case "confirmed":
		return red(status)
	case "dismissed":
		return dim(status)
	default:
		return yellow(status)
	}
}

// cmdAnomaliesReview records an operator's own judgement about one
// already-flagged event. It takes no action on the account itself —
// no lock, no revoke — there's nothing here to trigger, which is what
// keeps this inside the read-only rule rather than being an exception
// to it in spirit. See api's own admin.mdx for the same reasoning.
func cmdAnomaliesReview(cfg csaxConfig, eventID, status, note string) {
	if !apiMode(cfg) {
		fmt.Println("`csax anomalies` requires API-client mode — set CSAX_API_URL.")
		os.Exit(1)
	}
	if status != "unreviewed" && status != "confirmed" && status != "dismissed" {
		fmt.Println(red("invalid status: must be unreviewed, confirmed, or dismissed"))
		os.Exit(1)
	}
	client, err := newAPIClient(cfg)
	if err != nil {
		fmt.Println(red("error: " + err.Error()))
		os.Exit(1)
	}

	body := map[string]string{"status": status}
	if note != "" {
		body["note"] = note
	}
	var resp struct {
		Status string `json:"status"`
	}
	if err := client.put(cfg, "/admin/anomalies/"+eventID, body, &resp); err != nil {
		fmt.Println(red("request failed: " + err.Error()))
		os.Exit(1)
	}
	fmt.Printf("Recorded: %s is now %s\n", eventID, anomalyStatusColor(resp.Status))
}
