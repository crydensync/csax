# csax

Admin CLI for CrydenSync — manage users, sessions, and audit logs from the terminal. Not end-user facing; this is for developers/operators running a CrydenSync-backed app, same as `psql` is for a database, not for the app's own users.

## Installation

```bash
go install github.com/crydensync/csax@latest
```

If you get `command not found` (or, on Windows, `'csax' is not recognized`) after this, `csax` installed correctly — it's just not on your `PATH` yet. Fix:

**Linux / macOS / Termux:**
```bash
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc   # use ~/.zshrc if that's your shell
source ~/.bashrc
```

**Windows (PowerShell):**
```powershell
setx PATH "$env:Path;$(go env GOPATH)\bin"
```
Then open a new terminal window for it to take effect.

Confirm it worked:
```bash
csax version
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
csax stats                                                  # total users, active sessions, etc.
csax health
csax version
```

## Design notes

- Every command uses either the engine's public API/store methods, or — for a small number of read-only, system-wide commands the engine's store interfaces don't support (`users list`, `stats`, `audit search`) — direct SQL against the known Postgres schema, the same way `csax migrate` already does. No CrydenSync engine Go code was modified or added specifically to support the CLI.
- `MIGRATIONS_DIR` (default `./migrations`) should point at a folder containing both CrydenSync's own migration files and your app's own — `csax migrate` treats them the same, just files matching `*.up.sql`/`*.down.sql`, run in filename order.
- No CLI framework dependency (no Cobra) — deliberately dependency-light, same philosophy as the engine itself.
- Colored output by default (auto-disabled when not writing to a real terminal). Commands returning structured data (`users get`, `users list`, `sessions list`) support `--json` for scripting.

## License

MIT
