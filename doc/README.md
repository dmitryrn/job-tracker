# Nice

Local SQLite-backed job discovery, beginning with Adzuna. It runs as a
long-lived server that checks whether each provider is due to synchronize on
startup and every minute. Its HTTP API serves the job browser and a
downloadable database.

## Configuration

Create `config.toml` from `config.toml.example`. The application does not
supply defaults; Jobicy's `geo`, `industry`, and `tag` filters are optional:

```toml
[server]
http_address = ":8080"

[database]
path = "jobs.db"

[providers.adzuna]
query = "software engineer"
country = "de"
max_days_old = 30
max_pages = 5
results_per_page = 50
workplace = "remote-hybrid"
sync_interval = "6h"

[providers.remotive]
query = "software engineer"
sync_interval = "6h"

[providers.jobicy]
count = 50
geo = "europe"
industry = "engineering"
tag = ""
sync_interval = "6h"
```

Create `.env.tokens` from `.env.tokens.example` and place the Adzuna
credentials there:

```dotenv
ADZUNA_APP_ID=your-app-id
ADZUNA_API_KEY=your-api-key
```

Register for both values at <https://developer.adzuna.com/signup>.

## Sync

Adzuna has one marketplace per country, so set `ADZUNA_COUNTRY` to an
Adzuna-supported code such as `de`, `fr`, `gb`, or `pl`, rather than `rs`.

```sh
go run ./cmd/server
```

The server requests the configured number of Adzuna result pages, filters for
the configured workplace preference, and searches each provider using its own
configured query. Each provider has its own minimum sync interval; the database
records every attempt before it starts, so restarting the server does not
trigger an early repeat request. Both sources are upserted into SQLite with
their advertised company names. Set `providers.adzuna.workplace` to `any`,
`remote`, or `remote-hybrid`.

Adzuna provides a description snippet, while Remotive and Jobicy provide full
HTML job descriptions. Jobicy accepts up to 200 results and can be filtered by
an optional `geo`, `industry`, or `tag`; leave a filter blank to omit it. Look
up current geography and industry slugs from Jobicy before adding a filter. Its
sync interval must be at least one hour. The database keeps every source URL so
an interesting role can be opened at its source.

## API

JSON endpoints permit cross-origin requests from the UI.

| Method | Path | Description |
| --- | --- | --- |
| `GET` | `/api/jobs?search=&provider=&fields=` | Lists up to 100 newest jobs. `fields` is a comma-separated selection of `title`, `company`, `location`, and `body`; omitted searches all four. |
| `GET` | `/api/providers` | Lists job sources currently stored in the database. |
| `GET` | `/api/companies?search=` | Lists up to 100 companies and their stored job counts. |
| `DELETE` | `/api/jobs/{id}` | Deletes a stored job. A later sync can add it back if the provider still supplies it. |
| `GET` | `/api/database` | Downloads the SQLite database for the DB browser. |

An API error has the form `{"error":"message"}`. Deleting an existing job returns `204 No Content`; a missing job returns `404 Not Found`.
