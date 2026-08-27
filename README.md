# Nice

Local SQLite-backed job discovery, beginning with Adzuna. It runs as a
long-lived server that synchronizes jobs on startup and at a configured
interval. Its HTTP API serves the job browser and a downloadable database.

## Configuration

Create `.env` from `.env.example`. Every setting is required; the application
does not supply defaults:

```dotenv
HTTP_ADDRESS=:8080
DATABASE_PATH=jobs.db
ADZUNA_COUNTRY=de
JOB_QUERY=software engineer
ADZUNA_MAX_DAYS_OLD=30
ADZUNA_MAX_PAGES=5
ADZUNA_RESULTS_PER_PAGE=50
ADZUNA_WORKPLACE=remote-hybrid
SYNC_INTERVAL=6h
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
the configured workplace preference, and also searches Remotive using
`JOB_QUERY`. Both sources are upserted into SQLite with their advertised
company names. Set `ADZUNA_WORKPLACE` to `any`, `remote`, or `remote-hybrid`.

Adzuna provides a description snippet, not a complete job description.
Remotive provides full HTML job descriptions. The database keeps every source
URL so an interesting role can be opened at its source.

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
