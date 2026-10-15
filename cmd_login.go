package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// cmdLogin logs in to a deployed api instance as an operator and
// caches the session, so later commands (Tier 3 onward) don't ask for
// credentials again. This command's whole job in this tier is proving
// the round trip end to end — it deliberately does nothing else yet.
func cmdLogin(cfg csaxConfig) {
	if !apiMode(cfg) {
		fmt.Fprintln(os.Stderr, "CSAX_API_URL is not set — `csax login` only applies to API-client mode. Direct-DB mode (the default) needs no login at all.")
		os.Exit(1)
	}

	client, err := newAPIClient(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	token, err := client.ensureLoggedIn(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "login failed:", err)
		os.Exit(1)
	}

	// Confirm the token actually round-trips against the deployment,
	// not just that login itself returned something.
	var verifyResp struct {
		UserID string `json:"user_id"`
	}
	if err := client.get(cfg, "/v1/verify", &verifyResp); err != nil {
		fmt.Fprintln(os.Stderr, "logged in, but the token failed to verify:", err)
		os.Exit(1)
	}

	role := roleFromToken(token)
	fmt.Printf("Logged in to %s as %s\n", cfg.APIURL, verifyResp.UserID)
	if role == "" {
		fmt.Println("This account has no admin role — API-client mode's admin commands (Tier 3 onward) will fail with not_operator until one is granted.")
	} else {
		fmt.Printf("Role: %s\n", role)
	}
}

// roleFromToken reads the "role" claim out of the access token's own
// JWT payload for display purposes only — this is not a trust
// decision. The server verifies the token's signature and the role
// claim on every real admin request regardless of what csax shows
// here; a tampered local display value can't grant anything.
func roleFromToken(accessToken string) string {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.Role
}

// cmdLogout clears the cached session — csax's equivalent of the
// deployment's own /v1/logout, but purely local: it does not call the
// deployment at all, since a cached refresh token an operator wants
// gone should be forgotten immediately, not on a best-effort network
// call that could fail.
func cmdLogout() {
	if err := clearSessionCache(); err != nil {
		fmt.Fprintln(os.Stderr, "error clearing cached session:", err)
		os.Exit(1)
	}
	fmt.Println("Cleared cached session.")
}
