import { useEffect, useState, type FormEvent } from "react";
import { fetchEvents, type EventPage } from "./api";

const pageSize = 50;

function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

export default function EventsView() {
  const [provider, setProvider] = useState("");
  const [runID, setRunID] = useState("");
  const [type, setType] = useState("");
  const [level, setLevel] = useState("");
  const [activeProvider, setActiveProvider] = useState("");
  const [activeRunID, setActiveRunID] = useState("");
  const [activeType, setActiveType] = useState("");
  const [activeLevel, setActiveLevel] = useState("");
  const [offset, setOffset] = useState(0);
  const [reload, setReload] = useState(0);
  const [page, setPage] = useState<EventPage>({ events: [], total: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      setLoading(true);
      try {
        const result = await fetchEvents(activeProvider, activeRunID, activeType, activeLevel, pageSize, offset, controller.signal);
        if (!controller.signal.aborted) {
          setPage(result);
          setError("");
        }
      } catch (reason) {
        if (!controller.signal.aborted) {
          setError(reason instanceof Error ? reason.message : "Could not load events");
        }
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      }
    }
    void load();
    return () => controller.abort();
  }, [activeProvider, activeRunID, activeType, activeLevel, offset, reload]);

  function filter(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setActiveProvider(provider);
    setActiveRunID(runID);
    setActiveType(type);
    setActiveLevel(level);
    setOffset(0);
    setReload((current) => current + 1);
  }

  const pageStart = page.total === 0 ? 0 : offset + 1;
  const pageEnd = Math.min(offset + page.events.length, page.total);

  return (
    <section className="events-page">
      <header className="events-header">
        <div>
          <p className="eyebrow">Observability</p>
          <h1>Application events</h1>
          <p>LinkedIn runs include the search and individual listing requests that produced them. Use a run ID to inspect one sync from start to finish.</p>
        </div>
        <p className="events-total">{page.total} events</p>
      </header>

      <form className="events-filter" onSubmit={filter}>
        <label>Source<select value={provider} onChange={(event) => setProvider(event.target.value)}><option value="">All events</option><option value="application">Application</option><option value="linkedin">LinkedIn</option></select></label>
        <label>Run ID<input value={runID} onChange={(event) => setRunID(event.target.value)} placeholder="linkedin-..." /></label>
        <label>Event type<input value={type} onChange={(event) => setType(event.target.value)} placeholder="skipped" /></label>
        <label>Level<select value={level} onChange={(event) => setLevel(event.target.value)}><option value="">All levels</option><option value="info">Info</option><option value="error">Error</option></select></label>
        <button className="secondary-action">Apply filters</button>
      </form>

      {error && <p className="query-error">{error}</p>}
      {loading ? <p className="browse-loading">Loading events...</p> : page.events.length === 0 ? <p className="profile-empty">No events match these filters.</p> : (
        <ol className="events-list">
          {page.events.map((event) => (
            <li className={`event-card ${event.level}`} key={event.id}>
              <div className="event-card-header">
                <div>
                  <p className="eyebrow">{event.provider || "application"} · {event.type}</p>
                </div>
                <time dateTime={event.occurredAt}>{formatTime(event.occurredAt)}</time>
              </div>
              <div className="event-meta">
                {event.runId && <span>Run: <code>{event.runId}</code></span>}
                <span>{event.level}</span>
              </div>
              {Object.keys(event.data).length > 0 && <pre>{JSON.stringify(event.data, null, 2)}</pre>}
            </li>
          ))}
        </ol>
      )}

      <footer className="events-pagination">
        <span>Showing {pageStart}-{pageEnd} of {page.total}</span>
        <div>
          <button className="secondary-action" disabled={offset === 0 || loading} onClick={() => setOffset((current) => Math.max(0, current - pageSize))}>Previous</button>
          <button className="secondary-action" disabled={offset + page.events.length >= page.total || loading} onClick={() => setOffset((current) => current + pageSize)}>Next</button>
        </div>
      </footer>
    </section>
  );
}
