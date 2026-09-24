package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// This file's two commands are honestly NOT output-identical to their
// direct-DB counterparts, unlike cmd_users_api.go — api's
// GET /admin/oauth/health reports one "configured" bool per provider,
// not separately whether the client ID or the client secret is what's
// missing, so that finer distinction (which direct-DB mode's `list`
// shows) simply isn't knowable through this endpoint. In exchange,
// api's health check does a real, live reachability probe for every
// provider in a single call — something direct-DB mode's `list` never
// did at all, only its separate `test` command did per-provider. Both
// modes are genuinely useful, they're just not the same shape, and
// pretending otherwise would hide real information either way.

type apiOAuthHealthEntry struct {
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
	Status     string `json:"status"`
	HTTPStatus int    `json:"http_status"`
	LatencyMS  int    `json:"latency_ms"`
}

func fetchOAuthHealth(cfg csaxConfig) ([]apiOAuthHealthEntry, error) {
	client, err := newAPIClient(cfg)
	if err != nil {
		return nil, err
	}
	var entries []apiOAuthHealthEntry
	if err := client.get(cfg, "/admin/oauth/health", &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func cmdOAuthProvidersListAPI(cfg csaxConfig, jsonOutput bool) {
	entries, err := fetchOAuthHealth(cfg)
	if err != nil {
		fmt.Println(red("request failed: " + err.Error()))
		os.Exit(1)
	}

	if jsonOutput {
		b, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(b))
		return
	}

	columns := []string{"PROVIDER", "CONFIGURED", "STATUS", "HTTP", "LATENCY"}
	var rows [][]string
	for _, e := range entries {
		httpCol, latencyCol := "-", "-"
		if e.HTTPStatus != 0 {
			httpCol = fmt.Sprintf("%d", e.HTTPStatus)
		}
		if e.LatencyMS != 0 {
			latencyCol = fmt.Sprintf("%dms", e.LatencyMS)
		}
		rows = append(rows, []string{e.Provider, yesNo(e.Configured), oauthStatusColor(e.Status), httpCol, latencyCol})
	}
	printTable(columns, rows)
	fmt.Println(dim("\nStatus and reachability are live, from the deployment's own perspective — csax never contacted any provider directly for this."))
}

func oauthStatusColor(status string) string {
	switch status {
	case "ok":
		return green(status)
	case "degraded", "unreachable":
		return red(status)
	default:
		return dim(status)
	}
}

// cmdOAuthTestAPI is one entry from the same health call, filtered to
// the provider asked for — api's health check has no concept of
// testing "one provider on demand" separate from checking all of
// them, so this doesn't cost any less than providers list itself.
func cmdOAuthTestAPI(cfg csaxConfig, providerName string) {
	entries, err := fetchOAuthHealth(cfg)
	if err != nil {
		fmt.Println(red("request failed: " + err.Error()))
		os.Exit(1)
	}

	for _, e := range entries {
		if e.Provider != providerName {
			continue
		}
		fmt.Printf("Provider:   %s\n", e.Provider)
		fmt.Printf("Configured: %s\n", yesNo(e.Configured))
		fmt.Printf("Status:     %s\n", oauthStatusColor(e.Status))
		if e.HTTPStatus != 0 {
			fmt.Printf("HTTP:       %d\n", e.HTTPStatus)
		}
		if e.LatencyMS != 0 {
			fmt.Printf("Latency:    %dms\n", e.LatencyMS)
		}
		fmt.Println(dim("\nThis is api's own live reachability probe, not a redirect_uri check — csax cannot verify a redirect_uri is registered correctly through this endpoint the way direct-DB mode's test does."))
		if e.Status != "ok" {
			os.Exit(1)
		}
		return
	}
	fmt.Println(red("unknown or unconfigured provider: " + providerName))
	os.Exit(1)
}
