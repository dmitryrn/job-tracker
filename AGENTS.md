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
- add spacing between code blocks as per clean code, for readability
- handlers should log all errors, including service errors
- run tests, build after making code changes
- verify Go builds with `go build -o /dev/null ./cmd/server` to avoid build artifacts
- live analyzer fixtures may use available free OpenRouter models; authentication loads from `.env.tokens` at runtime
- when restarting a service, make sure it works, check logs, etc
