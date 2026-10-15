package main

// New surface, not an extension of an existing command — cryden's
// weekly digest, login diagnosis, and config tuning advisor had no
// csax command at all before this. Both modes make sense here, unlike
// most of this PR's other additions: cryden.WeeklyDigest/DigestSince,
// cryden.DiagnoseLoginIssue, and cryden.ConfigTuningReport/
// TuningReportSince are plain engine functions taking nothing but the
// *cryden.Engine direct-DB mode already builds for other commands, so
// there's no reason to make these API-client-only the way the
// anomaly review queue in Tier 7 has to be.
//
// digest history has no direct-DB equivalent at all — cryden's own
// WeeklyDigest only ever answers on demand, there is no engine-level
// concept of scheduled history to read locally. That one sub-command
// is API-client-only, and says so plainly rather than doing nothing.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/crydensync/cryden/v2"
)

func cmdDigest(cfg csaxConfig, db *sql.DB, sinceDuration time.Duration, jsonOutput bool) {
	engine := mustBuildEngine(cfg, db)
	ctx := context.Background()

	var text string
	var err error
	if sinceDuration > 0 {
		text, err = cryden.DigestSince(ctx, engine, time.Now().Add(-sinceDuration))
	} else {
		text, err = cryden.WeeklyDigest(ctx, engine)
	}
	if err != nil {
		fmt.Println(red("failed to build digest: " + err.Error()))
		os.Exit(1)
	}
	printReportText(text, jsonOutput)
}

func cmdDigestAPI(cfg csaxConfig, sinceDays int, jsonOutput bool) {
	client, err := newAPIClient(cfg)
	if err != nil {
		fmt.Println(red("error: " + err.Error()))
		os.Exit(1)
	}
	var resp struct {
		Text string `json:"text"`
	}
	path := "/admin/digest"
	if sinceDays > 0 {
		path = fmt.Sprintf("/admin/digest?window_days=%d", sinceDays)
	}
	if err := client.get(cfg, path, &resp); err != nil {
		fmt.Println(red("request failed: " + err.Error()))
		os.Exit(1)
	}
	printReportText(resp.Text, jsonOutput)
}

// cmdDigestHistoryAPI has no direct-DB counterpart at all — see this
// file's own top-of-file comment for why. Callers reach this only
// through API-client mode; main.go refuses it outright otherwise.
func cmdDigestHistoryAPI(cfg csaxConfig, limit int, jsonOutput bool) {
	client, err := newAPIClient(cfg)
	if err != nil {
		fmt.Println(red("error: " + err.Error()))
		os.Exit(1)
	}
	var entries []struct {
		Text      string `json:"text"`
		CreatedAt string `json:"created_at"`
	}
	if err := client.get(cfg, fmt.Sprintf("/admin/digest/history?limit=%d", limit), &entries); err != nil {
		fmt.Println(red("request failed: " + err.Error()))
		os.Exit(1)
	}
	if jsonOutput {
		b, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(b))
		return
	}
	if len(entries) == 0 {
		fmt.Println(dim("No digest history recorded yet — this deployment may not have a scheduled digest job configured."))
		return
	}
	for i, e := range entries {
		if i > 0 {
			fmt.Println(dim("\n---"))
		}
		fmt.Println(dim(e.CreatedAt))
		fmt.Println(e.Text)
	}
}

func cmdSupportDiagnose(cfg csaxConfig, db *sql.DB, email string, jsonOutput bool) {
	engine := mustBuildEngine(cfg, db)
	text, err := cryden.DiagnoseLoginIssue(context.Background(), engine, email)
	if err != nil {
		fmt.Println(red("diagnosis failed: " + err.Error()))
		os.Exit(1)
	}
	printReportText(text, jsonOutput)
}

func cmdSupportDiagnoseAPI(cfg csaxConfig, email string, jsonOutput bool) {
	client, err := newAPIClient(cfg)
	if err != nil {
		fmt.Println(red("error: " + err.Error()))
		os.Exit(1)
	}
	var resp struct {
		Text string `json:"text"`
	}
	if err := client.get(cfg, "/admin/support/diagnose?email="+email, &resp); err != nil {
		fmt.Println(red("request failed: " + err.Error()))
		os.Exit(1)
	}
	printReportText(resp.Text, jsonOutput)
}

func cmdConfigTuning(cfg csaxConfig, db *sql.DB, windowDays int, jsonOutput bool) {
	engine := mustBuildEngine(cfg, db)
	ctx := context.Background()

	var text string
	var err error
	if windowDays > 0 {
		text, err = cryden.TuningReportSince(ctx, engine, time.Now().Add(-time.Duration(windowDays)*24*time.Hour))
	} else {
		text, err = cryden.ConfigTuningReport(ctx, engine)
	}
	if err != nil {
		fmt.Println(red("failed to build config tuning report: " + err.Error()))
		os.Exit(1)
	}
	printReportText(text, jsonOutput)
}

// cmdConfigTuningAPI renders api's structured suggestion list the
// same way the direct-DB text report reads, so the two modes don't
// look like different tools — see api's own admin.mdx: this endpoint
// returns {area, finding, suggestion} triples, not pre-rendered text.
func cmdConfigTuningAPI(cfg csaxConfig, windowDays int, jsonOutput bool) {
	client, err := newAPIClient(cfg)
	if err != nil {
		fmt.Println(red("error: " + err.Error()))
		os.Exit(1)
	}
	var resp struct {
		Suggestions []struct {
			Area       string `json:"area"`
			Finding    string `json:"finding"`
			Suggestion string `json:"suggestion"`
		} `json:"suggestions"`
	}
	path := "/admin/config-tuning"
	if windowDays > 0 {
		path = fmt.Sprintf("/admin/config-tuning?window_days=%d", windowDays)
	}
	if err := client.get(cfg, path, &resp); err != nil {
		fmt.Println(red("request failed: " + err.Error()))
		os.Exit(1)
	}

	if jsonOutput {
		b, _ := json.MarshalIndent(resp.Suggestions, "", "  ")
		fmt.Println(string(b))
		return
	}
	if len(resp.Suggestions) == 0 {
		fmt.Println(dim("Nothing stands out against current settings in this window."))
		return
	}
	for _, s := range resp.Suggestions {
		fmt.Println(yellow(s.Area))
		fmt.Println("  " + s.Finding)
		fmt.Println("  → " + s.Suggestion)
		fmt.Println()
	}
}

// printReportText is shared by every command in this file that just
// gets back one block of prose — the only thing that differs between
// them is where the text came from.
func printReportText(text string, jsonOutput bool) {
	if jsonOutput {
		b, _ := json.Marshal(map[string]string{"text": text})
		fmt.Println(string(b))
		return
	}
	fmt.Println(text)
}
