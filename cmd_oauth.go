package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// oauthProviderStatus mirrors what api's own provider() switch checks
// — kept independent (not imported from api, they're separate repos)
// but deliberately using the exact same env var names, so this
// command reports on the real values production actually uses.
type oauthProviderStatus struct {
	Name               string
	ClientIDSet        bool
	ClientSecretSet    bool
	TokenURL           string
	discoveryCheckAddr string // used by cmdOAuthTest only
}

func oauthProviders(cfg csaxConfig) []oauthProviderStatus {
	return []oauthProviderStatus{
		{
			Name:               "google",
			ClientIDSet:        cfg.GoogleClientID != "",
			ClientSecretSet:    cfg.GoogleClientSecret != "",
			TokenURL:           "https://oauth2.googleapis.com/token",
			discoveryCheckAddr: "https://accounts.google.com/.well-known/openid-configuration",
		},
		{
			Name:               "github",
			ClientIDSet:        cfg.GitHubClientID != "",
			ClientSecretSet:    cfg.GitHubClientSecret != "",
			TokenURL:           "https://github.com/login/oauth/access_token",
			discoveryCheckAddr: "https://github.com/login/oauth/authorize",
		},
	}
}

func cmdOAuthProvidersList(cfg csaxConfig, jsonOutput bool) {
	providers := oauthProviders(cfg)

	if jsonOutput {
		out := make([]map[string]any, 0, len(providers))
		for _, p := range providers {
			configured := p.ClientIDSet && p.ClientSecretSet
			out = append(out, map[string]any{
				"provider":          p.Name,
				"configured":        configured,
				"client_id_set":     p.ClientIDSet,
				"client_secret_set": p.ClientSecretSet,
			})
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Printf("%-10s %-12s %-16s %-18s\n", "PROVIDER", "CONFIGURED", "CLIENT ID SET", "CLIENT SECRET SET")
	for _, p := range providers {
		configured := p.ClientIDSet && p.ClientSecretSet
		configuredStr := yesNo(configured)
		if configured {
			configuredStr = green(configuredStr)
		} else {
			configuredStr = dim(configuredStr)
		}
		fmt.Printf("%-10s %-12s %-16s %-18s\n", p.Name, configuredStr, yesNo(p.ClientIDSet), yesNo(p.ClientSecretSet))
	}
	if cfg.BaseURL == "" {
		fmt.Println(yellow("\nwarning: BASE_URL is not set — provider redirect URIs cannot be computed correctly"))
	}
}

// cmdOAuthTest round-trips a provider's real endpoints using the
// actual configured client ID/secret/redirect URI, WITHOUT a live
// user. This exists because OAuth's most common failure — a
// redirect-URI mismatch — otherwise only surfaces in production
// against a real person mid-login.
func cmdOAuthTest(cfg csaxConfig, providerName string) {
	providers := oauthProviders(cfg)
	var p *oauthProviderStatus
	for i := range providers {
		if providers[i].Name == providerName {
			p = &providers[i]
			break
		}
	}
	if p == nil {
		fmt.Println(red("unknown provider: " + providerName + " (expected: google, github)"))
		os.Exit(1)
	}

	fmt.Printf("Testing %s...\n\n", p.Name)
	ok := true

	if p.ClientIDSet && p.ClientSecretSet {
		fmt.Println(green("✓") + " client ID and secret present")
	} else {
		fmt.Println(red("✗") + " client ID and/or secret missing")
		ok = false
	}

	if cfg.BaseURL == "" {
		fmt.Println(red("✗") + " BASE_URL is not set — cannot compute a redirect URI")
		ok = false
	} else {
		redirectURI := cfg.BaseURL + "/v1/oauth/" + p.Name + "/callback"
		fmt.Println(dim("  redirect_uri would be: " + redirectURI))
		fmt.Println(yellow("  ⚠ csax cannot verify this matches what's registered in " + p.Name + "'s console — check that manually"))
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(p.discoveryCheckAddr)
	if err != nil {
		fmt.Println(red("✗") + " could not reach " + p.Name + "'s endpoint: " + err.Error())
		ok = false
	} else {
		resp.Body.Close()
		// A reachable response — even a 4xx, since we're not sending
		// real credentials on this GET — proves network/DNS/TLS all
		// work, which is the actual thing worth checking here before
		// a real user hits it.
		fmt.Println(green("✓") + " reached " + p.Name + "'s endpoint")
	}

	fmt.Println()
	if ok {
		fmt.Println(green("This provider looks ready. The one thing csax cannot verify automatically is whether the redirect_uri above is registered exactly as shown in " + p.Name + "'s console — mismatches there are the #1 real-world OAuth failure."))
	} else {
		fmt.Println(red("This provider is not ready yet — see the ✗ items above."))
		os.Exit(1)
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
