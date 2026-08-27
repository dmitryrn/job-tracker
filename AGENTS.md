# Credentials

Do not read, display, edit, or commit `.env.tokens`.

The application loads `.env.tokens` at runtime. Use `.env.tokens.example` to
document required credential names without accessing real values.

# Services

The application server runs as the per-user systemd unit `jobs.service`, and
the UI runs as the per-user unit `jobs-ui.service`; neither is a global system
service. Do not start `go run ./cmd/server` manually or stop its process
directly. Use `systemctl --user` to inspect or restart either service and
`journalctl --user -u jobs.service` to view backend logs. When no user session
environment is present, set `XDG_RUNTIME_DIR=/run/user/1002` and
`DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1002/bus` for the `clanker` user.

# Rules
- don't hallucinate lib versions when adding
- add spacing between code blocks as per clean code, for readability
- handlers should log all errors, including service errors
- run tests, build after making changes