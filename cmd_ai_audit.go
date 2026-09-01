package main

import (
	"context"
	"fmt"
)

// auditFinding is produced entirely by plain Go logic below — the
// model never decides what counts as a finding, it only explains and
// prioritizes findings that already exist. This is the deliberate
// split from the design: "the checklist itself is fixed Go code, not
// model-generated."
type auditFinding struct {
	Severity string // "HIGH", "MEDIUM", "OK"
	Message  string
}

// runAuditChecklist is pure, deterministic, and needs no AI provider
// to run — worth being able to run standalone later if `csax ai
// audit` ever gets a non-AI sibling command.
func runAuditChecklist(cfg csaxConfig) []auditFinding {
	var findings []auditFinding

	if cfg.JWTSecret == "" {
		findings = append(findings, auditFinding{"HIGH", "JWT_SECRET is not set."})
	} else if len(cfg.JWTSecret) < 32 {
		findings = append(findings, auditFinding{"HIGH", fmt.Sprintf("JWT secret is %d characters — recommend 32+ for HS256.", len(cfg.JWTSecret))})
	} else {
		findings = append(findings, auditFinding{"OK", "JWT secret length looks reasonable."})
	}

	if cfg.ReadOnlyDBURL == "" {
		findings = append(findings, auditFinding{"MEDIUM", "READONLY_DATABASE_URL is not set — ai query/logs are unavailable until it is."})
	} else if cfg.ReadOnlyDBURL == cfg.DatabaseURL {
		findings = append(findings, auditFinding{"HIGH", "READONLY_DATABASE_URL is identical to DATABASE_URL — this must point at a genuinely read-only role, not the same writable connection."})
	} else {
		findings = append(findings, auditFinding{"OK", "A separate read-only database connection is configured."})
	}

	if cfg.BaseURL == "" {
		findings = append(findings, auditFinding{"MEDIUM", "BASE_URL is not set — OAuth callback URLs cannot be computed."})
	} else {
		findings = append(findings, auditFinding{"OK", "BASE_URL is set."})
	}

	oauthConfigured := (cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "") || (cfg.GitHubClientID != "" && cfg.GitHubClientSecret != "")
	if !oauthConfigured {
		findings = append(findings, auditFinding{"OK", "No OAuth providers configured — nothing to check there yet."})
	}

	return findings
}

// cmdAIAudit implements `csax ai audit`. No --fix flag, deliberately
// — per the standing constraint, if one is ever added it must show a
// diff and require per-change confirmation, never a batch apply.
// This command only ever prints; it changes nothing.
func cmdAIAudit(cfg csaxConfig) {
	findings := runAuditChecklist(cfg)

	for _, f := range findings {
		icon := "✓"
		color := green
		if f.Severity == "HIGH" {
			icon, color = "⚠", red
		} else if f.Severity == "MEDIUM" {
			icon, color = "⚠", yellow
		}
		fmt.Printf("%s %-6s %s\n", color(icon), f.Severity, f.Message)
	}

	flagged := 0
	for _, f := range findings {
		if f.Severity != "OK" {
			flagged++
		}
	}
	fmt.Printf("\n%d check(s), %d flagged. No changes made.\n", len(findings), flagged)

	// The narrative layer is optional — audit still works, and still
	// changes nothing, even if AI isn't configured at all.
	provider, err := newLLMProvider(cfg)
	if err != nil || flagged == 0 {
		return
	}
	var narrative string
	summarizeErr := withSpinner("Summarizing...", func() error {
		var serr error
		narrative, serr = provider.Summarize(context.Background(),
			"You are prioritizing a fixed list of security configuration findings for a system administrator. "+
				"You do not add new findings or invent facts not present in the list. Explain, in 2-3 sentences, "+
				"which flagged item to fix first and why, in plain language.",
			formatFindingsForSummary(findings))
		return serr
	})
	if summarizeErr == nil && narrative != "" {
		fmt.Println("\n" + narrative)
	}
}

func formatFindingsForSummary(findings []auditFinding) string {
	out := ""
	for _, f := range findings {
		out += f.Severity + ": " + f.Message + "\n"
	}
	return out
}
