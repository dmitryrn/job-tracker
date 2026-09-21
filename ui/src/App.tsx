import { useEffect, useRef, useState } from "react";
import initSqlJs, { type Database } from "sql.js";
import sqlWasm from "sql.js/dist/sql-wasm.wasm?url";
import BrowseView, { type BrowseMode } from "./BrowseView";
import ApplicationsView from "./ApplicationsView";
import DiscoverySettingsView from "./DiscoverySettingsView";
import EventsView from "./EventsView";
import JobDetailView from "./JobDetailView";
import JobMatchesView from "./JobMatchesView";
import MatchQueueView from "./MatchQueueView";
import ProfileView from "./ProfileView";
import ResumeView from "./ResumeView";
import { apiURL, fetchJob, type BrowseJob } from "./api";
import { inspectSchema, queryTable, rowLimit, type Rows, type Sort, type Table } from "./database";
import "./styles.css";

type View = BrowseMode | "applications" | "events" | "explorer" | "matches" | "profile" | "resume" | "match-queue" | "search-setup";

type Route = {
  view: View;
  jobID?: number;
	tab?: "post" | "match" | "chat";
};

function readRoute(): Route {
  const parts = window.location.pathname.split("/").filter(Boolean);
  if (parts[0] === "jobs" && /^\d+$/.test(parts[1] ?? "")) {
		return { view: "jobs", jobID: Number(parts[1]), tab: parts[2] === "match" || parts[2] === "chat" ? parts[2] : "post" };
  }
  if (parts[0] === "applications" || parts[0] === "companies" || parts[0] === "events" || parts[0] === "matches" || parts[0] === "profile" || parts[0] === "resume" || parts[0] === "match-queue" || parts[0] === "search-setup" || parts[0] === "database") {
    return { view: parts[0] === "database" ? "explorer" : parts[0] };
  }
  return { view: "jobs" };
}

function routePath(route: Route) {
  if (route.jobID) {
		return `/jobs/${route.jobID}${route.tab === "post" ? "" : `/${route.tab ?? "post"}`}`;
  }
  return route.view === "explorer" ? "/database" : `/${route.view}`;
}

function databaseURL() {
  const configured = import.meta.env.VITE_DATABASE_URL;
  if (configured) {
    return configured;
  }
  return apiURL("database");
}

function displayValue(value: unknown) {
  if (value === null) {
    return "NULL";
  }
  if (value instanceof Uint8Array) {
    return `[binary: ${value.byteLength} bytes]`;
  }
  return String(value);
}

function formattedValue(value: unknown) {
  if (typeof value !== "string") {
    return displayValue(value);
  }

  try {
    const parsed = JSON.parse(value);
    if (parsed !== null && typeof parsed === "object") {
      return JSON.stringify(parsed, null, 2);
    }
  } catch {
    // Plain text values should be shown exactly as stored.
  }

  if (/<[a-z][\s\S]*>/i.test(value)) {
    return new DOMParser().parseFromString(value, "text/html").body.innerText;
  }

  return value;
}

export default function App() {
  const [database, setDatabase] = useState<Database>();
  const [tables, setTables] = useState<Table[]>([]);
  const [tableSearch, setTableSearch] = useState("");
  const [selectedTable, setSelectedTable] = useState<Table>();
  const [rows, setRows] = useState<Rows>({ columns: [], values: [] });
  const [filter, setFilter] = useState("");
  const [sort, setSort] = useState<Sort>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [selectedValue, setSelectedValue] = useState<{ column: string; value: unknown }>();
  const [route, setRoute] = useState<Route>(readRoute);
  const [selectedJob, setSelectedJob] = useState<BrowseJob>();
  const [jobLoading, setJobLoading] = useState(false);
  const [jobError, setJobError] = useState("");
  const valueDialog = useRef<HTMLDialogElement>(null);

  function navigate(nextRoute: Route) {
    const path = routePath(nextRoute);
    if (window.location.pathname !== path) {
      window.history.pushState({}, "", path);
    }
    setRoute(nextRoute);
  }

  useEffect(() => {
    function onPopState() {
      setRoute(readRoute());
    }
    window.addEventListener("popstate", onPopState);
    if (window.location.pathname === "/") {
      window.history.replaceState({}, "", "/jobs");
      setRoute({ view: "jobs" });
    }
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  useEffect(() => {
    if (route.view !== "explorer" || database) {
      return;
    }
    let active = true;
    let opened: Database | undefined;
    setLoading(true);
    setError("");

    async function loadDatabase() {
      try {
        const [SQL, response] = await Promise.all([
          initSqlJs({ locateFile: () => sqlWasm }),
          fetch(databaseURL()),
        ]);
        if (!response.ok) {
          throw new Error(`Database download failed (${response.status})`);
        }
        opened = new SQL.Database(new Uint8Array(await response.arrayBuffer()));
        const discovered = inspectSchema(opened);
        if (!active) {
          opened.close();
          return;
        }
        setDatabase(opened);
        setTables(discovered);
        setSelectedTable(discovered[0]);
        opened = undefined;
      } catch (reason) {
        if (active) {
          setError(reason instanceof Error ? reason.message : "Could not open the database");
        }
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    }

    void loadDatabase();
    return () => {
      active = false;
      opened?.close();
    };
  }, [database, route.view]);

  useEffect(() => {
    if (!route.jobID) {
      setSelectedJob(undefined);
      setJobError("");
      return;
    }
    const controller = new AbortController();
    async function loadJob() {
      setJobLoading(true);
      try {
        const result = await fetchJob(route.jobID!, controller.signal);
        if (!controller.signal.aborted) {
          setSelectedJob(result.job);
          setJobError("");
        }
      } catch (reason) {
        if (!controller.signal.aborted) {
          setSelectedJob(undefined);
          setJobError(reason instanceof Error ? reason.message : "Could not load job");
        }
      } finally {
        if (!controller.signal.aborted) {
          setJobLoading(false);
        }
      }
    }
    void loadJob();
    return () => controller.abort();
  }, [route.jobID]);

  useEffect(() => {
    if (!database || !selectedTable) {
      return;
    }
    try {
      setRows(queryTable(database, selectedTable.name, filter, sort));
      setError("");
    } catch (reason) {
      setRows({ columns: [], values: [] });
      setError(reason instanceof Error ? reason.message : "The filter could not be run");
    }
  }, [database, selectedTable, filter, sort]);

  useEffect(() => {
    const dialog = valueDialog.current;
    if (selectedValue && dialog && !dialog.open) {
      dialog.showModal();
    }
  }, [selectedValue]);

  function selectTable(table: Table) {
    setSelectedTable(table);
    setFilter("");
    setSort(undefined);
  }

  function toggleSort(column: string) {
    setSort((current) =>
      current?.column === column
        ? { column, direction: current.direction === "ASC" ? "DESC" : "ASC" }
        : { column, direction: "ASC" },
    );
  }

  const visibleTables = tables.filter((table) => table.name.toLowerCase().includes(tableSearch.trim().toLowerCase()));

  if (route.view === "explorer" && !loading && (!database || error && !selectedTable)) {
    return <main className="state error">{error || "The database could not be opened."}</main>;
  }

  return (
    <main className={route.view === "explorer" ? "shell" : "shell browse-layout"}>
      <header className="app-header">
        <nav className="app-nav" aria-label="Application navigation">
          <button className={route.view === "jobs" || route.view === "companies" ? "app-nav-link active" : "app-nav-link"} onClick={() => navigate({ view: "jobs" })}>Jobs</button>
           <button className={route.view === "matches" ? "app-nav-link active" : "app-nav-link"} onClick={() => navigate({ view: "matches" })}>Matches</button>
           <button className={route.view === "applications" ? "app-nav-link active" : "app-nav-link"} onClick={() => navigate({ view: "applications" })}>Applications</button>
          <button className={route.view === "match-queue" ? "app-nav-link active" : "app-nav-link"} onClick={() => navigate({ view: "match-queue" })}>Match queue</button>
           <button className={route.view === "profile" ? "app-nav-link active" : "app-nav-link"} onClick={() => navigate({ view: "profile" })}>Profile</button>
           <button className={route.view === "resume" ? "app-nav-link active" : "app-nav-link"} onClick={() => navigate({ view: "resume" })}>Resume</button>
          <button className={route.view === "search-setup" ? "app-nav-link active" : "app-nav-link"} onClick={() => navigate({ view: "search-setup" })}>Search setup</button>
          <button className={route.view === "events" ? "app-nav-link active" : "app-nav-link"} onClick={() => navigate({ view: "events" })}>Events</button>
          <button className={route.view === "explorer" ? "app-nav-link active" : "app-nav-link"} onClick={() => navigate({ view: "explorer" })}>DB browser</button>
        </nav>
      </header>
      {route.view === "explorer" && database && (
        <aside className="sidebar">
          <p className="eyebrow">Schema</p>
          <label className="table-search" htmlFor="table-search">
            <span>Search tables</span>
            <input
              id="table-search"
              type="search"
              value={tableSearch}
              onChange={(event) => setTableSearch(event.target.value)}
              placeholder="Table name"
            />
          </label>
          <nav className="table-list" aria-label="Database tables">
            {visibleTables.map((table) => (
            <button
              className={table.name === selectedTable?.name ? "table-link active" : "table-link"}
              key={table.name}
              onClick={() => selectTable(table)}
            >
              <span>{table.name}</span>
            </button>
          ))}
          </nav>
          {visibleTables.length === 0 && <p className="table-search-empty">No tables match.</p>}
        </aside>
      )}

      <section className="workspace">
        {route.jobID ? (
          jobLoading ? <p className="state browse-state">Loading job...</p> : selectedJob ? <JobDetailView job={selectedJob} tab={route.tab ?? "post"} onTabChange={(tab) => navigate({ ...route, tab })} onBack={() => navigate({ view: "jobs" })} onDeleted={() => navigate({ view: "jobs" })} /> : <p className="state error">{jobError || "Job not found."}</p>
        ) : route.view === "jobs" || route.view === "companies" ? (
          <BrowseView mode={route.view} onModeChange={(view) => navigate({ view })} onOpenJob={(job) => navigate({ view: "jobs", jobID: job.id, tab: "post" })} />
        ) : route.view === "match-queue" ? (
          <MatchQueueView onOpenJob={(job) => navigate({ view: "jobs", jobID: job.id, tab: "post" })} />
         ) : route.view === "matches" ? (
           <JobMatchesView onOpenMatch={(job) => navigate({ view: "jobs", jobID: job.id, tab: "match" })} />
         ) : route.view === "applications" ? (
           <ApplicationsView onOpenJob={(job) => navigate({ view: "jobs", jobID: job.id, tab: "match" })} />
         ) : route.view === "profile" ? (
           <ProfileView />
         ) : route.view === "resume" ? (
           <ResumeView />
        ) : route.view === "search-setup" ? (
          <DiscoverySettingsView />
        ) : route.view === "events" ? (
          <EventsView />
        ) : loading || !database ? (
          <p className="state browse-state">Downloading and opening the SQLite database...</p>
        ) : (
        <>
        <header>
          <div>
            <p className="eyebrow">{selectedTable?.kind}</p>
            <h1>{selectedTable?.name}</h1>
          </div>
          <p className="limit">Showing up to {rowLimit} rows. Queries run in this browser.</p>
        </header>

        <section className="columns" aria-label="Table columns">
          {selectedTable?.columns.map((column) => (
            <div className="column-card" key={column.name}>
              <strong>{column.name}</strong>
              <span>{column.type}</span>
              {column.primaryKey && <small>primary key</small>}
              {!column.primaryKey && !column.nullable && <small>required</small>}
            </div>
          ))}
        </section>

        <form className="filter" onSubmit={(event) => event.preventDefault()}>
          <label htmlFor="filter">Filter expression</label>
          <input
            id="filter"
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder="e.g. company_id IN (SELECT id FROM companies WHERE name LIKE '%studio%')"
          />
          <button type="button" onClick={() => setFilter("")}>Clear</button>
        </form>

        {error && <p className="query-error">{error}</p>}
        <div className="grid-wrap">
          <table>
            <thead>
              <tr>
                {rows.columns.map((column) => (
                  <th key={column}>
                    <button onClick={() => toggleSort(column)}>
                      {column}{sort?.column === column ? (sort.direction === "ASC" ? " ^" : " v") : ""}
                    </button>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.values.map((row, index) => (
                <tr key={index}>
                  {row.map((value, valueIndex) => {
                    const column = rows.columns[valueIndex];
                    return (
                      <td key={column}>
                        <button
                          className="cell-value"
                          onClick={() => setSelectedValue({ column, value })}
                          title={`Open ${column}`}
                        >
                          {displayValue(value)}
                        </button>
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
          {!error && rows.values.length === 0 && <p className="empty">No rows match this filter.</p>}
        </div>

        {selectedValue && (
          <dialog
            aria-labelledby="value-dialog-title"
            className="value-dialog"
            onCancel={() => setSelectedValue(undefined)}
            onClick={(event) => {
              if (event.target === event.currentTarget) {
                setSelectedValue(undefined);
              }
            }}
            ref={valueDialog}
          >
            <div className="value-dialog-header">
              <div>
                <p className="eyebrow">Field value</p>
                <h2 id="value-dialog-title">{selectedValue.column}</h2>
              </div>
              <button aria-label="Close field value" className="dialog-close" onClick={() => setSelectedValue(undefined)}>
                Close
              </button>
            </div>
            <pre>{formattedValue(selectedValue.value)}</pre>
         </dialog>
        )}
        </>
        )}
      </section>
    </main>
  );
}
