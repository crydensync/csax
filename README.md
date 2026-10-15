# csax

Admin CLI for CrydenSync — manage users, sessions, and audit logs from the terminal. Not end-user facing; this is for developers/operators running a CrydenSync-backed app, same as `psql` is for a database, not for the app's own users.

## Install

```bash
go install github.com/crydensync/csax@latest
```

## Setup

```bash
csax config init
```

Prompts for your database connection string and JWT secret, writes `.env`. Same DB your CrydenSync-backed app already uses.

## Commands

```bash
csax migrate up|down|status                      # run/track CrydenSync + your app's own migrations
csax users list [--limit N] [--offset N] [--json]  # every user, newest first
csax users get <email> [--json]                      # user details, lock status, active session count
csax users create <email> <password>                  # create a user directly
csax users unlock <email>                              # clear a lockout early
csax sessions list --user <email> [--json]
csax sessions revoke <session-id> --user <email>
csax sessions revoke-all --user <email>
csax audit tail --user <email> [--limit N]
csax audit search --event <type> [--limit N]              # system-wide, across all users
csax audit ask "<question>"                                # alias for `ai ask` — natural-language question over your own data
csax oauth providers list [--json] | add <provider>         # add: interactive, configures one provider's credentials
csax oauth test <provider>                                   # round-trips the provider's real endpoints before a live user hits it
csax oauth users get <email> [--json]                        # which providers an account has linked
csax oauth unlink <email> --provider <name>                  # force-unlink a provider from an account
csax oauth config                                            # interactive: BASE_URL/FRONTEND_URL + both providers at once
csax ai query "<natural language>" [--json]                  # read-only, allowlisted lookups over users/sessions/audit_events
csax ai logs "<natural language>"                            # natural-language audit event search, summarized — never acts
csax ai anomalies scan [--since 24h]                         # fixed-prompt `ai logs`, looks for credential-stuffing/abuse patterns
csax ai ask "<question>"                                     # direct question over any allowlisted entity, answered in plain language
csax ai audit                                                # flags likely misconfigurations from a fixed checklist — never auto-applies anything
csax ai config                                               # interactive: provider/model/API key + auto-creates the read-only DB role
csax doctor                                                 # one-shot health check: DB, migrations, AI config, OAuth config
csax stats                                                  # total users, active sessions, etc.
csax health
csax version
```

## Optional: OAuth admin commands

`oauth providers list`/`add`/`test` all read the SAME env vars `api`
uses for its own OAuth config — they check the real values production
uses, not a separate csax-only copy:

```
BASE_URL, FRONTEND_URL, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET
```

Run `csax oauth config` for an interactive setup of all of the above,
or `csax oauth providers add <provider>` to add just one provider once
`BASE_URL`/`FRONTEND_URL` are already set.

**`BASE_URL` vs `FRONTEND_URL` — a mixup that actually happened during
testing:** `BASE_URL` is your BACKEND's own public URL, since that's
where the `/api/oauth/.../callback` route lives. If your frontend and
backend are on different domains (e.g. Vercel + Railway), `BASE_URL`
is the Railway one, not the Vercel one. Both wizards spell this out
explicitly rather than assuming it's obvious.

`oauth users get`/`oauth unlink` manage which providers a specific
user has linked. These don't call any `cryden` engine method — they
query `oauth_identities` directly via SQL, same pattern as `users
list`/`stats` below. No engine change was needed or made to support
them.

## Optional: AI-assisted admin commands

Run `csax ai config` for an interactive setup — it prompts for the
provider/model/API-key-env-name, and can create the read-only
database role for you directly (see below), rather than handing you a
script to run by hand. Or set these in `.env` yourself:

```
AI_PROVIDER=groq                # or "openrouter" — both speak the same OpenAI-compatible chat completions shape
AI_API_KEY_ENV=GROQ_API_KEY     # the NAME of the env var holding your key — the key itself lives wherever that env var is set, never in .env
AI_MODEL=...                    # any model id your chosen provider serves
READONLY_DATABASE_URL=...       # a SEPARATE connection string, pointing at a Postgres role that is physically read-only
```

`READONLY_DATABASE_URL` is the real safety boundary for `ai
query`/`ai logs`/`ai anomalies scan`/`ai ask` — even a bug in the
underlying allowlist validation can't cause a write if the credential
itself is incapable of one. Don't point it at the same role
`DATABASE_URL` uses.

`csax ai config`'s role setup works against ANY Postgres provider, not
just Supabase — it only appends Supabase's `<role>.<project-ref>`
username suffix when it actually detects a Supabase pooler host; a
self-hosted Postgres, Neon, RDS, etc. all just get the plain role
name.

`ai audit` works even without any AI config — the checklist itself is
plain Go, not model-generated; the model is only used to add a short
prioritization narrative on top, which is skipped silently if AI isn't
configured.

None of the `ai` commands ever execute an action a finding surfaces —
`ai audit` never applies a fix, `ai logs`/`ai anomalies scan` never
revoke a session or lock an account they flag. Anything like that is a
suggestion in the output text, run yourself as a separate, explicit
command.

**Privacy note on `ai ask`/`ai logs`/`ai anomalies scan`:** emails and
IPs are redacted to stable per-query placeholders (`email_1`, `ip_2`)
before anything is sent to your configured AI provider for
summarization — patterns like repetition and clustering are still
visible to the model, but real values never leave your infrastructure
through that path. The full, unredacted data is still what gets
printed to your own terminal afterward. This is a default `csax`
ships with, not something separately audited — worth knowing if
you're evaluating this for a deployment with stricter data-handling
requirements.

## Design notes

- Every command uses either the engine's public API/store methods, or — for a small number of read-only, system-wide commands the engine's store interfaces don't support (`users list`, `stats`, `audit search`, `oauth users get`, `oauth unlink`) — direct SQL against the known Postgres schema, the same way `csax migrate` already does. No CrydenSync engine Go code was modified or added specifically to support the CLI.
- `MIGRATIONS_DIR` (default `./migrations`) should point at a folder containing both CrydenSync's own migration files and your app's own — `csax migrate` treats them the same, just files matching `*.up.sql`/`*.down.sql`, run in filename order.
- No CLI framework dependency (no Cobra) — deliberately dependency-light, same philosophy as the engine itself. This is also why `oauth`/`ai` help text lives in `usage()` by hand rather than being generated — there's no framework here to generate it from.
- `ai` commands are the one part of csax that calls out to something other than this deployment's own Postgres — the `ai.LLMProvider` interface ships zero implementations upstream in `cryden`; csax brings its own (`llmProvider` in `aiprovider.go`, an OpenAI-chat-completions-shaped client — works with Groq, OpenRouter, or any other provider speaking that same shape via `AI_PROVIDER`), same pattern as `notify.EmailSender`.
- Colored, box-drawing table output by default (`table.go`) — auto-disabled when not writing to a real terminal, or when `NO_COLOR` is set (see https://no-color.org). Long values (UUIDs, emails) are truncated with `…` rather than blowing up column widths. Audit event types get semantic color: red for anything `*_failed`/`*_reuse_detected`/`*_locked`, green for `*_success`/`*_linked`/`*_unlocked`.
- A terminal spinner (`spinner.go`) shows during any real network/DB round trip — AI provider calls, the OAuth endpoint reachability check, and the read-only role's connection test. Skipped automatically in the same non-TTY/`NO_COLOR` cases as the table output, so scripted/piped usage never sees spinner frames mixed into captured output.
- `csax doctor` aggregates the existing `csax health` and `csax ai audit` checks plus an OAuth config summary into one command — it calls the same underlying functions those commands use rather than re-implementing the checks a second time.
- **Known gap, not addressed this release:** `--json` is only implemented on a handful of commands (`users get`, `users list`, `oauth providers list`, `oauth users get`, `ai query`). Retrofitting it consistently across every command (`ai logs`, `ai ask`, `ai anomalies scan`, `ai audit`, `doctor`, `stats`, `audit tail`/`search`) is real, not-yet-done work — each needs its own JSON-serializable shape, not just a flag.
- **Known gap, not addressed this release:** this is still one flat `package main` across ~20 files. Splitting into proper subpackages (`internal/oauth`, `internal/ai`, `internal/config`, ...) is a real structural change touching nearly every file's imports at once — deliberately NOT attempted alongside this release's feature work, since it's a much higher-risk change to make at the same time as everything else here, and much harder to review as one large diff. Worth doing as its own dedicated pass, with nothing else changing in that same commit.

## License

MIT
