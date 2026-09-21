# Credentials

Do not read, display, edit, or commit `.env.tokens`.

The application loads `.env.tokens` at runtime. Use `.env.tokens.example` to
document required credential names without accessing real values.

# Services

The application server runs as the per-user systemd unit `jobs.service`, and
the UI runs as the per-user unit `jobs-ui.service`; neither is a global system
service. Do not start `go run ./cmd/server` manually or stop its process
directly. Use `systemctl --user` to inspect or restart either service and
`journalctl --user -u jobs.service` to view backend logs.

`jobs-ui.service` has a `Wants=` and `After=` dependency on `jobs.service`, so
starting the UI also starts the backend and waits for its startup ordering.

`~/.config/systemd/user/jobs.service` and
`~/.config/systemd/user/jobs-ui.service` are symlinks to
`systemd/jobs.service` and `systemd/jobs-ui.service` in this repository.
Edit those tracked files, then run `systemctl --user daemon-reload` after
changing a unit definition. Read recent service logs with
`journalctl --user-unit=jobs.service -n 30 --no-pager` or
`journalctl --user-unit=jobs-ui.service -n 30 --no-pager`.

# Rules
- don't hallucinate lib versions when adding
- handlers should log all errors, including service errors
- worker execution paths should log all outcomes, including early returns, successes, and errors, with relevant IDs and context
- clients should be dumb and application-agnostic where possible; retries and application policy belong to callers, though clients may use application models when needed
- callers sending requests to an LLM must pass a session header or equivalent session identifier when the provider supports one, preserving upstream cache affinity
- LLM retries, pipelines, and chats must use append-only context: preserve existing messages exactly, especially the initial user prompt, and append responses or correction feedback rather than rewriting prior context; reuse the session identifier within a retry loop to preserve prefix-cache eligibility
- keep each repository interface and its persistence implementation in one dedicated file under `internal/repositories`; do not combine unrelated repositories in one file
- use Squirrel builders for all new SQL queries
- format SQL column lists and scan/value argument lists with one field per line
- prefer narrow function parameters over passing whole structs when only a few fields are used
- tests should assert behavior, not log output
- run linter, tests, build after making code changes
- use `task wsl-fix` for `wsl_v5` auto-fixes the same way you would use `gofmt` for formatting cleanup
- verify Go builds with `go build -o /dev/null ./cmd/server` to avoid build artifacts
- live analyzer fixtures may use available free OpenRouter models; authentication loads from `.env.tokens` at runtime
- when restarting a service, make sure it works, check logs, etc
- check sqlite db (in this dir) schema if needed
- do not fetch or curl Linkedin without first curling https://ipinfo.io and confirming this matchine is running through the ProtonVPN
