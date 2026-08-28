# Nice

Local SQLite-backed job discovery, beginning with Adzuna. It runs as a
long-lived server that checks whether each provider is due to synchronize on
startup and every minute. Its HTTP API serves the job browser and a
downloadable database.

## Configuration

Create local configuration from [`config.toml.example`](../config.toml.example)
and credentials from [`.env.tokens.example`](../.env.tokens.example). Those
files are the authoritative configuration reference.

## Sync

The application runs as the per-user `jobs.service` and `jobs-ui.service`
systemd units. Start the UI with:

```sh
systemctl --user start jobs-ui.service
```

## API

HTTP handlers and their route registrations are in [`internal/server`](../internal/server).
