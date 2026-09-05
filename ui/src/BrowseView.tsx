import { useEffect, useState } from "react";
import { fetchCompanies, fetchJobs, fetchMatchQueue, fetchProviders, queueUnmatchedJobMatches, type BrowseCompany, type BrowseJob } from "./api";

export type BrowseMode = "jobs" | "companies";

type BrowseViewProps = {
  mode: BrowseMode;
  onModeChange: (mode: BrowseMode) => void;
  onOpenJob: (job: BrowseJob) => void;
};

const searchFieldOptions = [
  { value: "title", label: "Title" },
  { value: "company", label: "Company" },
  { value: "location", label: "Location" },
  { value: "body", label: "Description" },
];

const pageSize = 50;

export function formatDate(value: string, now = new Date()) {
  if (!value) {
    return "Recently posted";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "Recently posted";
  }
  const time = new Intl.DateTimeFormat(undefined, { hour: "numeric", minute: "2-digit" }).format(date);
  const daysAgo = Math.floor((now.getTime() - date.getTime()) / (24 * 60 * 60 * 1000));
  if (daysAgo <= 0) {
    return `Today, ${time}`;
  }
  if (daysAgo <= 30) {
    return `${daysAgo}d ago, ${time}`;
  }
  return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", year: "numeric", hour: "numeric", minute: "2-digit" }).format(date);
}

function formatSalary(job: BrowseJob) {
  if (job.salaryMin === null && job.salaryMax === null) {
    return "Compensation not listed";
  }
  const formatter = new Intl.NumberFormat(undefined, { style: "currency", currency: "USD", maximumFractionDigits: 0 });
  if (job.salaryMin !== null && job.salaryMax !== null) {
    return `${formatter.format(job.salaryMin)} - ${formatter.format(job.salaryMax)}`;
  }
  return formatter.format(job.salaryMin ?? job.salaryMax ?? 0);
}

function formatProvider(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function plainText(value: string) {
  if (!/<[a-z][\s\S]*>/i.test(value)) {
    return value;
  }
  return new DOMParser().parseFromString(value, "text/html").body.innerText;
}

export function jobsReadyToQueue(jobs: BrowseJob[], queuedJobIDs: ReadonlySet<number>) {
  return jobs.filter((job) => !job.hasMatch && !queuedJobIDs.has(job.id));
}

function JobCard({ job, onOpen }: { job: BrowseJob; onOpen: () => void }) {
  return (
    <article className="job-card">
      <button className="job-card-button" onClick={onOpen}>
        <div className="job-card-topline">
          <span className="job-card-status"><span className="source-label">{job.source}</span><span className={job.hasMatch ? "match-status ready" : "match-status"}>{job.hasMatch ? "Match ready" : "No match yet"}</span></span>
          <span className="posted-label">{formatDate(job.postedAt)}</span>
        </div>
        <h2>{job.title}</h2>
        <p className="company-name">{job.company || "Company not listed"}</p>
        <p className="job-excerpt">{plainText(job.bodyText).replace(/\s+/g, " ").trim()}</p>
        <div className="job-meta">
          <span>{job.location || "Location flexible"}</span>
          <span>{job.employmentType || "Role type not listed"}</span>
        </div>
      </button>
      <div className="job-card-footer">
        <span>{formatSalary(job)}</span>
        <a href={job.sourceURL} target="_blank" rel="noreferrer">Open listing</a>
      </div>
    </article>
  );
}

function CompanyCard({ company }: { company: BrowseCompany }) {
  return (
    <article className="company-card">
      <div className="company-mark">{company.name.slice(0, 1).toUpperCase() || "?"}</div>
      <div>
        <h2>{company.name || "Company not listed"}</h2>
        <p>{company.jobCount} {company.jobCount === 1 ? "open role" : "open roles"}</p>
      </div>
    </article>
  );
}

export default function BrowseView({ mode, onModeChange, onOpenJob }: BrowseViewProps) {
  const [search, setSearch] = useState("");
  const [provider, setProvider] = useState("");
  const [match, setMatch] = useState("all");
  const [searchFields, setSearchFields] = useState(searchFieldOptions.map(({ value }) => value));
  const [providers, setProviders] = useState<string[]>([]);
  const [jobs, setJobs] = useState<BrowseJob[]>([]);
  const [companies, setCompanies] = useState<BrowseCompany[]>([]);
  const [queuedJobIDs, setQueuedJobIDs] = useState<Set<number>>(() => new Set());
  const [offset, setOffset] = useState(0);
  const [totalJobs, setTotalJobs] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [queueing, setQueueing] = useState(false);
  const [queueMessage, setQueueMessage] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    setQueueMessage("");
    async function load() {
      setLoading(true);
      try {
        if (mode === "jobs") {
          const [jobResult, providerResult, queueResult] = await Promise.all([
            fetchJobs(search, provider, match, searchFields, pageSize, offset, controller.signal),
            fetchProviders(controller.signal),
            fetchMatchQueue(controller.signal),
          ]);
          setJobs(jobResult.jobs);
          setTotalJobs(jobResult.total);
          setProviders(providerResult.providers);
          setQueuedJobIDs(new Set(queueResult.jobs.map((job) => job.id)));
        } else {
          const result = await fetchCompanies(search, controller.signal);
          setCompanies(result.companies);
        }
        setError("");
      } catch (reason) {
        if (reason instanceof DOMException && reason.name === "AbortError") {
          return;
        }
        setError(reason instanceof Error ? reason.message : "Could not load browse data");
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      }
    }
    void load();
    return () => controller.abort();
  }, [match, mode, offset, provider, search, searchFields]);

  function toggleSearchField(field: string, checked: boolean) {
    setOffset(0);
    setSearchFields((current) => checked ? [...current, field] : current.filter((value) => value !== field));
  }

  const unmatchedJobs = jobs.filter((job) => !job.hasMatch);
  const queueableJobs = jobsReadyToQueue(jobs, queuedJobIDs);
  const queuedUnmatchedJobs = unmatchedJobs.length - queueableJobs.length;

  async function queueUnmatchedJobs() {
    setQueueing(true);
    try {
      const result = await queueUnmatchedJobMatches(queueableJobs.map((job) => job.id));
      setQueuedJobIDs((current) => new Set([...current, ...queueableJobs.map((job) => job.id)]));
      setQueueMessage(result.queued > 0 ? `${result.queued} ${result.queued === 1 ? "role" : "roles"} queued for processing.` : "All unmatched roles in view are already queued.");
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not queue unmatched roles");
    } finally {
      setQueueing(false);
    }
  }

  return (
    <section className="browse-page">
      <header className="browse-header">
        <div>
          <p className="eyebrow">Browse / {mode}</p>
          {mode === "companies" && <h1>Meet the companies hiring</h1>}
          {mode === "companies" && <p className="browse-subtitle">The organizations behind the roles in your database.</p>}
        </div>
        <div className="browse-summary">
          <div className="browse-stat">
            <strong>{mode === "jobs" ? totalJobs : companies.length}</strong>
            <span>{mode === "jobs" ? "roles found" : "companies in view"}</span>
          </div>
          {mode === "jobs" && <button type="button" className="queue-unmatched-action" disabled={loading || queueing || queueableJobs.length === 0} onClick={() => void queueUnmatchedJobs()}>{queueing ? "Queueing..." : queueableJobs.length > 0 ? `Queue ${queueableJobs.length} unmatched ${queueableJobs.length === 1 ? "role" : "roles"}${queuedUnmatchedJobs > 0 ? ` (${queuedUnmatchedJobs} queued)` : ""}` : queuedUnmatchedJobs > 0 ? `All ${queuedUnmatchedJobs} unmatched ${queuedUnmatchedJobs === 1 ? "role is" : "roles are"} queued` : "No unmatched roles"}</button>}
        </div>
      </header>

      <nav className="browse-tabs" aria-label="Browse sections">
        <button className={mode === "jobs" ? "browse-tab active" : "browse-tab"} onClick={() => onModeChange("jobs")}>Jobs</button>
        <button className={mode === "companies" ? "browse-tab active" : "browse-tab"} onClick={() => onModeChange("companies")}>Companies</button>
      </nav>

      <form className="browse-search" onSubmit={(event) => event.preventDefault()}>
        <label htmlFor="browse-search">Search {mode}</label>
        <input
          id="browse-search"
          value={search}
          onChange={(event) => { setSearch(event.target.value); setOffset(0); }}
          placeholder={mode === "jobs" ? "Search titles, companies, or locations" : "Search company names"}
        />
        {mode === "jobs" && (
          <>
            <label htmlFor="provider">Provider</label>
            <select id="provider" value={provider} onChange={(event) => { setProvider(event.target.value); setOffset(0); }}>
              <option value="">All providers</option>
              {providers.map((value) => <option key={value} value={value}>{formatProvider(value)}</option>)}
            </select>
            <label htmlFor="match">Match</label>
            <select id="match" value={match} onChange={(event) => { setMatch(event.target.value); setOffset(0); }}>
              <option value="all">All</option>
              <option value="has">Has match</option>
              <option value="none">No match</option>
            </select>
          </>
        )}
        {mode === "jobs" && (
          <fieldset className="search-fields">
            <legend>Search in</legend>
            {searchFieldOptions.map(({ value, label }) => (
              <label key={value}>
                <input type="checkbox" checked={searchFields.includes(value)} onChange={(event) => toggleSearchField(value, event.target.checked)} />
                {label}
              </label>
            ))}
          </fieldset>
        )}
        {search && <button type="button" onClick={() => { setSearch(""); setOffset(0); }}>Clear</button>}
      </form>

      {error && <p className="query-error">{error}</p>}
      {queueMessage && <p className="queue-message">{queueMessage}</p>}
      {loading && <p className="browse-loading">Loading...</p>}
      {mode === "jobs" ? (
        <div className="job-grid">
          {jobs.map((job) => <JobCard key={job.id} job={job} onOpen={() => onOpenJob(job)} />)}
        </div>
      ) : (
        <div className="company-grid">
          {companies.map((company) => <CompanyCard key={company.id} company={company} />)}
        </div>
      )}
      {!loading && !error && mode === "jobs" && (
        <footer className="browse-pagination">
          <span>Showing {totalJobs === 0 ? 0 : offset + 1}-{Math.min(offset + jobs.length, totalJobs)} of {totalJobs}</span>
          <div>
            <button type="button" className="secondary-action" disabled={offset === 0} onClick={() => setOffset((current) => Math.max(0, current - pageSize))}>Previous</button>
            <button type="button" className="secondary-action" disabled={offset + jobs.length >= totalJobs} onClick={() => setOffset((current) => current + pageSize)}>Next</button>
          </div>
        </footer>
      )}
      {!loading && !error && mode === "jobs" && jobs.length === 0 && <p className="empty browse-empty">No roles match this search.</p>}
      {!loading && !error && mode === "companies" && companies.length === 0 && <p className="empty browse-empty">No companies match this search.</p>}
    </section>
  );
}
