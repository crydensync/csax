package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// This file mirrors cmd_users_list.go/cmd_users.go's *output shapes*
// exactly, on purpose — a script running `csax users list --json`
// must get the same fields whether the deployment is reached over
// Postgres or over api's HTTP admin surface. Only the internal query
// path differs.
//
// users create/unlock have no equivalent here at all — see
// CLAUDE.md's "two reach modes" section for why that's permanent,
// not a gap this file is expected to eventually close.

// apiUserRecord is the subset of api's GET /admin/users response this
// file actually uses — api returns more fields (updated_at,
// locked_until, failed_attempts) but cmdUsersListAPI only surfaces
// what cmdUsersList's own JSON shape already promises, to keep that
// shape identical between modes.
type apiUserRecord struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Created string `json:"created_at"`
	Locked  bool   `json:"locked"`
}

// cmdUsersListAPI is users list's API-client path. api's own search
// has no plain browse-everyone mode distinct from a query — an empty
// q parameter is how this asks for one; see api's admin.mdx.
func cmdUsersListAPI(cfg csaxConfig, limit, offset int, jsonOutput bool) {
	client, err := newAPIClient(cfg)
	if err != nil {
		fmt.Println(red("error: " + err.Error()))
		os.Exit(1)
	}

	var resp struct {
		Users []apiUserRecord `json:"users"`
		Total int             `json:"total"`
	}
	path := fmt.Sprintf("/admin/users?limit=%d&offset=%d", limit, offset)
	if err := client.get(cfg, path, &resp); err != nil {
		fmt.Println(red("request failed: " + err.Error()))
		os.Exit(1)
	}

	type userRow struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		CreatedAt string `json:"created_at"`
		Locked    bool   `json:"locked"`
	}
	users := make([]userRow, 0, len(resp.Users))
	for _, u := range resp.Users {
		users = append(users, userRow{ID: u.ID, Email: u.Email, CreatedAt: u.Created, Locked: u.Locked})
	}

	if jsonOutput {
		b, _ := json.MarshalIndent(users, "", "  ")
		fmt.Println(string(b))
		return
	}

	if len(users) == 0 {
		fmt.Println(dim("No users found."))
		return
	}
	for _, u := range users {
		lockLabel := ""
		if u.Locked {
			lockLabel = yellow(" [locked]")
		}
		fmt.Printf("%s  %-40s %s%s\n", u.ID, u.Email, u.CreatedAt, lockLabel)
	}
	fmt.Println(dim(fmt.Sprintf("\nShowing %d of %d (limit=%d offset=%d) — use --limit/--offset to page.", len(users), resp.Total, limit, offset)))
}

// cmdUsersGetAPI is users get's API-client path. Two round trips
// where direct-DB mode needed one query: a search for the exact
// email, then a detail fetch for the account it finds — that's the
// accepted cost of not holding a database credential.
func cmdUsersGetAPI(cfg csaxConfig, email string, jsonOutput bool) {
	client, err := newAPIClient(cfg)
	if err != nil {
		fmt.Println(red("error: " + err.Error()))
		os.Exit(1)
	}

	var searchResp struct {
		Users []apiUserRecord `json:"users"`
		Match string          `json:"match"`
	}
	if err := client.get(cfg, "/admin/users?q="+email, &searchResp); err != nil {
		fmt.Println(red("request failed: " + err.Error()))
		os.Exit(1)
	}
	if len(searchResp.Users) == 0 {
		fmt.Println(red("failed to find user: no account matches " + email))
		os.Exit(1)
	}
	found := searchResp.Users[0]

	var detail struct {
		User struct {
			LockedUntil    *string `json:"locked_until"`
			FailedAttempts int     `json:"failed_attempts"`
		} `json:"user"`
		ActiveSessions int `json:"active_sessions"`
	}
	if err := client.get(cfg, "/admin/users/"+found.ID, &detail); err != nil {
		fmt.Println(red("request failed: " + err.Error()))
		os.Exit(1)
	}

	if jsonOutput {
		out := map[string]any{
			"id":              found.ID,
			"email":           found.Email,
			"created_at":      found.Created,
			"locked":          found.Locked,
			"locked_until":    detail.User.LockedUntil,
			"failed_attempts": detail.User.FailedAttempts,
			"active_sessions": detail.ActiveSessions,
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Printf("ID:       %s\n", found.ID)
	fmt.Printf("Email:    %s\n", found.Email)
	fmt.Printf("Created:  %s\n", found.Created)
	if detail.User.LockedUntil != nil {
		fmt.Println(yellow(fmt.Sprintf("Locked:   yes, until %s", *detail.User.LockedUntil)))
	} else {
		fmt.Printf("Locked:   no (failed attempts: %d)\n", detail.User.FailedAttempts)
	}
	fmt.Printf("Sessions: %d active\n", detail.ActiveSessions)
}
