import { useEffect, useState } from "react";
import { fetchApplications, type ApplicationSummary, type BrowseJob } from "./api";

type ApplicationsViewProps = {
  onOpenJob: (job: BrowseJob) => void;
};

const pageSizeOptions = [25, 50];

function applicationSearchFromParams(parameters: URLSearchParams) {
  const limit = Number(parameters.get("limit"));
  const offset = Number(parameters.get("offset"));
  return {
    pageSize: pageSizeOptions.includes(limit) ? limit : 25,
    offset: Number.isInteger(offset) && offset >= 0 ? offset : 0,
  };
}

function applicationSearchPath(pageSize: number, offset: number, pathname = window.location.pathname) {
  const parameters = new URLSearchParams();
  if (pageSize !== 25) parameters.set("limit", String(pageSize));
  if (offset !== 0) parameters.set("offset", String(offset));
  const query = parameters.toString();
  return query ? `${pathname}?${query}` : pathname;
}

function formatAppliedAt(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "Applied recently" : `Applied ${new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(date)}`;
}

export default function ApplicationsView({ onOpenJob }: ApplicationsViewProps) {
  const initial = applicationSearchFromParams(new URLSearchParams(window.location.search));
  const [applications, setApplications] = useState<ApplicationSummary[]>([]);
  const [total, setTotal] = useState(0);
  const [pageSize, setPageSize] = useState(initial.pageSize);
  const [offset, setOffset] = useState(initial.offset);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  function updatePage(nextPageSize: number, nextOffset: number) {
    window.history.pushState({}, "", applicationSearchPath(nextPageSize, nextOffset));
    setPageSize(nextPageSize);
    setOffset(nextOffset);
  }

  useEffect(() => {
    function onPopState() {
      const next = applicationSearchFromParams(new URLSearchParams(window.location.search));
      setPageSize(next.pageSize);
      setOffset(next.offset);
    }
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      setLoading(true);
      try {
        const result = await fetchApplications(pageSize, offset, controller.signal);
        if (!controller.signal.aborted) {
          setApplications(result.applications);
          setTotal(result.total);
          setError("");
        }
      } catch (reason) {
        if (!(reason instanceof DOMException && reason.name === "AbortError")) {
          setError(reason instanceof Error ? reason.message : "Could not load applications");
        }
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [offset, pageSize]);

  return (
    <section className="applications-page">
      <header className="applications-header">
        <div><p className="eyebrow">Applications</p><h1>Jobs you applied to</h1><p>Keep track of roles you have submitted.</p></div>
      </header>
      {error && <p className="query-error">{error}</p>}
      {loading && <p className="browse-loading">Loading applications...</p>}
      {!loading && !error && applications.length === 0 && <p className="empty browse-empty">No applications yet.</p>}
      {applications.length > 0 && (
        <ol className="applications-list">
          {applications.map((application) => (
            <li key={application.job.id}>
              <button type="button" className="application-job" onClick={() => onOpenJob(application.job)}>
                <span className="application-topline"><span>{formatAppliedAt(application.appliedAt)}</span><span>{application.job.source}</span></span>
                <strong>{application.job.title || "Untitled job"}</strong>
                <span>{application.job.company || "Company not listed"}</span>
              </button>
            </li>
          ))}
        </ol>
      )}
      {!loading && !error && (
        <footer className="browse-pagination">
          <span>Showing {total === 0 ? 0 : offset + 1}-{Math.min(offset + applications.length, total)} of {total}</span>
          <div>
            <label className="browse-page-size" htmlFor="applications-page-size">Applications per page
              <select id="applications-page-size" value={pageSize} onChange={(event) => updatePage(Number(event.target.value), 0)}>
                {pageSizeOptions.map((size) => <option key={size} value={size}>{size}</option>)}
              </select>
            </label>
            <button type="button" className="secondary-action" disabled={offset === 0} onClick={() => updatePage(pageSize, Math.max(0, offset - pageSize))}>Previous</button>
            <button type="button" className="secondary-action" disabled={offset + applications.length >= total} onClick={() => updatePage(pageSize, offset + pageSize)}>Next</button>
          </div>
        </footer>
      )}
    </section>
  );
}
