package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// cmdOAuthConfig interactively sets up OAuth env vars. Deliberately
// explicit about the BASE_URL vs FRONTEND_URL distinction — this is
// exactly the mixup that broke a real deployment during testing
// (BASE_URL was set to the frontend's Vercel URL instead of the
// backend's own URL, since that's where the callback ROUTE actually
// lives), so the prompts spell out which is which rather than
// assuming it's obvious.
func cmdOAuthConfig(cfg csaxConfig) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Setting up OAuth (Google/GitHub login).")
	fmt.Println()
	fmt.Println("BASE_URL is your BACKEND's own public URL — the callback route")
	fmt.Println("(/api/oauth/.../callback) is a route on YOUR SERVER, not on your")
	fmt.Println("frontend. If your frontend and backend are on different domains")
	fmt.Println("(e.g. Vercel + Railway), BASE_URL is the Railway one.")
	fmt.Println()

	baseURL := promptDefault(reader, "BASE_URL (your backend's own URL)", cfg.BaseURL)
	frontendURL := promptDefault(reader, "FRONTEND_URL (where the browser lands after login)", cfg.FrontendURL)

	values := map[string]string{
		"BASE_URL":     baseURL,
		"FRONTEND_URL": frontendURL,
	}

	if promptYesNo(reader, "Configure Google?", true) {
		values["GOOGLE_CLIENT_ID"] = promptDefault(reader, "Google client ID", "")
		values["GOOGLE_CLIENT_SECRET"] = promptDefault(reader, "Google client secret", "")
		printCallbackURLs(baseURL, "google")
	}
	if promptYesNo(reader, "Configure GitHub?", true) {
		values["GITHUB_CLIENT_ID"] = promptDefault(reader, "GitHub client ID", "")
		values["GITHUB_CLIENT_SECRET"] = promptDefault(reader, "GitHub client secret", "")
		printCallbackURLs(baseURL, "github")
	}

	appendEnvValues(reader, values)
	fmt.Println()
	fmt.Println("Register the callback URLs printed above in each provider's console before testing.")
	fmt.Println("Run `csax oauth test <provider>` afterward to confirm it's reachable.")
}

func printCallbackURLs(baseURL, provider string) {
	base := strings.TrimRight(baseURL, "/")
	fmt.Printf("\n  Register these in %s's console:\n", capitalizeFirst(provider))
	fmt.Printf("    %s/api/oauth/%s/callback\n", base, provider)
	fmt.Printf("    %s/api/oauth/%s/link/callback\n\n", base, provider)
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
