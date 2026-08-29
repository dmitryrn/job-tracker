import { useEffect, useRef, useState } from "react";
import initSqlJs, { type Database } from "sql.js";
import sqlWasm from "sql.js/dist/sql-wasm.wasm?url";
import BrowseView, { type BrowseMode } from "./BrowseView";
import JobDetailView from "./JobDetailView";
import ProfileView from "./ProfileView";
import type { BrowseJob } from "./api";
import { inspectSchema, queryTable, rowLimit, type Rows, type Sort, type Table } from "./database";
import "./styles.css";

type View = BrowseMode | "explorer" | "profile" | "job";

function databaseURL() {
  const configured = import.meta.env.VITE_DATABASE_URL;
  if (configured) {
    return configured;
  }
  return `${window.location.protocol}//${window.location.hostname}:4001/api/database`;
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
  const [selectedTable, setSelectedTable] = useState<Table>();
  const [rows, setRows] = useState<Rows>({ columns: [], values: [] });
  const [filter, setFilter] = useState("");
  const [sort, setSort] = useState<Sort>();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [selectedValue, setSelectedValue] = useState<{ column: string; value: unknown }>();
  const [view, setView] = useState<View>("jobs");
  const [selectedJob, setSelectedJob] = useState<BrowseJob>();
  const valueDialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    if (view !== "explorer" || database) {
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
  }, [database, view]);

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

  if (view === "explorer" && !loading && (!database || error && !selectedTable)) {
    return <main className="state error">{error || "The database could not be opened."}</main>;
  }

  return (
    <main className={view === "explorer" ? "shell" : "shell browse-layout"}>
      <header className="app-header">
        <div className="brand"><span>J</span><div><strong>Jobs</strong><small>Remote work, better sorted</small></div></div>
        <nav className="app-nav" aria-label="Application navigation">
            <button className={view === "jobs" || view === "companies" || view === "job" ? "app-nav-link active" : "app-nav-link"} onClick={() => setView("jobs")}>Jobs</button>
            <button className={view === "profile" ? "app-nav-link active" : "app-nav-link"} onClick={() => setView("profile")}>Profile</button>
            <button className={view === "explorer" ? "app-nav-link active" : "app-nav-link"} onClick={() => setView("explorer")}>DB browser</button>
        </nav>
      </header>
      {view === "explorer" && database && (
        <aside className="sidebar">
        <p className="eyebrow">Schema</p>
        <nav aria-label="Database tables">
          {tables.map((table) => (
            <button
              className={table.name === selectedTable?.name ? "table-link active" : "table-link"}
              key={table.name}
              onClick={() => selectTable(table)}
            >
              <span>{table.name}</span>
            </button>
          ))}
        </nav>
        </aside>
      )}

      <section className="workspace">
        {view === "jobs" || view === "companies" ? (
          <BrowseView mode={view} onModeChange={setView} onOpenJob={(job) => { setSelectedJob(job); setView("job"); }} />
        ) : view === "profile" ? (
          <ProfileView />
        ) : view === "job" && selectedJob ? (
          <JobDetailView job={selectedJob} onBack={() => setView("jobs")} onDeleted={() => { setSelectedJob(undefined); setView("jobs"); }} />
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
