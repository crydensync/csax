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
csax oauth providers list [--json]                          # which providers have client ID/secret set
csax oauth test <provider>                                   # round-trips the provider's real endpoints before a live user hits it
csax ai query "<natural language>" [--json]                  # read-only, allowlisted lookups over users/sessions/audit_events
csax ai logs "<natural language>"                            # natural-language audit event search, summarized — never acts
csax ai audit                                                # flags likely misconfigurations from a fixed checklist — never auto-applies anything
csax stats                                                  # total users, active sessions, etc.
csax health
csax version
```

## Optional: OAuth admin commands

`oauth providers list` and `oauth test` read the SAME env vars `api`
uses for its own OAuth config — they check the real values production
uses, not a separate csax-only copy:

```
BASE_URL, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET
```

`oauth users get`/`oauth unlink` (managing which providers a specific
user has linked) are designed but not built yet — they need two new
`cryden` engine facades that don't exist as of this CLI's current
`cryden` dependency version.

## Optional: AI-assisted admin commands

`ai query`/`ai logs`/`ai audit` need their own config, on top of the
usual `.env`:

```
AI_PROVIDER=groq                # or "openrouter" — both speak the same OpenAI-compatible chat completions shape
AI_API_KEY_ENV=GROQ_API_KEY     # the NAME of the env var holding your key — the key itself lives wherever that env var is set, never in .env
AI_MODEL=...                    # any model id your chosen provider serves
READONLY_DATABASE_URL=...       # a SEPARATE connection string, pointing at a Postgres role that is physically read-only
```

`READONLY_DATABASE_URL` is the real safety boundary for `ai query`/`ai
logs` — even a bug in the underlying allowlist validation can't cause
a write if the credential itself is incapable of one. Don't point it
at the same role `DATABASE_URL` uses.

`ai audit` works even without any AI config — the checklist itself is
plain Go, not model-generated; the model is only used to add a short
prioritization narrative on top, which is skipped silently if AI isn't
configured.

None of the `ai` commands ever execute an action a finding surfaces —
`ai audit` never applies a fix, and `ai logs` never revokes a session
or locks an account it flags. Anything like that is a suggestion in
the output text, run yourself as a separate, explicit command.

## Design notes

- Every command uses either the engine's public API/store methods, or — for a small number of read-only, system-wide commands the engine's store interfaces don't support (`users list`, `stats`, `audit search`) — direct SQL against the known Postgres schema, the same way `csax migrate` already does. No CrydenSync engine Go code was modified or added specifically to support the CLI.
- `MIGRATIONS_DIR` (default `./migrations`) should point at a folder containing both CrydenSync's own migration files and your app's own — `csax migrate` treats them the same, just files matching `*.up.sql`/`*.down.sql`, run in filename order.
- No CLI framework dependency (no Cobra) — deliberately dependency-light, same philosophy as the engine itself. This is also why `oauth`/`ai` help text lives in `usage()` by hand rather than being generated — there's no framework here to generate it from.
- `ai` commands are the one part of csax that calls out to something other than this deployment's own Postgres — the `ai.LLMProvider` interface ships zero implementations upstream in `cryden`; csax brings its own (`llmProvider` in `aiprovider.go`, an OpenAI-chat-completions-shaped client — works with Groq, OpenRouter, or any other provider speaking that same shape via `AI_PROVIDER`), same pattern as `notify.EmailSender`.
- Colored output by default (auto-disabled when not writing to a real terminal). Commands returning structured data (`users get`, `users list`, `sessions list`) support `--json` for scripting.

## License

MIT
