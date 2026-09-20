import { type FormEvent, useEffect, useRef, useState } from "react";
import { createCustomJob, fetchCompanies, fetchJobs, fetchMatchQueue, fetchProviders, queueUnmatchedJobMatches, type BrowseCompany, type BrowseJob } from "./api";
import { profileScoreClassName, profileScoreLabel, profileScoreStyle } from "./profileScore";

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

const pageSizeOptions = [10, 25, 50];

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
           <span className="job-card-status"><span className="source-label">{job.source}</span><span className={job.hasMatch ? "match-status ready" : "match-status"}>{job.hasMatch ? "Match ready" : "No match yet"}</span><span className={profileScoreClassName(job.profileMatchScore)} style={profileScoreStyle(job.profileMatchScore)}>Profile fit {profileScoreLabel(job.profileMatchScore)}</span></span>
          <span className="posted-label">{formatDate(job.postedAt)}</span>
        </div>
        <h2>{job.title || "Untitled job"}</h2>
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
  const [pageSize, setPageSize] = useState(25);
  const [offset, setOffset] = useState(0);
  const [totalJobs, setTotalJobs] = useState(0);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [queueing, setQueueing] = useState(false);
  const [queueMessage, setQueueMessage] = useState("");
  const [customJobOpen, setCustomJobOpen] = useState(false);
  const [customJobURL, setCustomJobURL] = useState("");
  const [creatingCustomJob, setCreatingCustomJob] = useState(false);
  const [refresh, setRefresh] = useState(0);
  const customJobDialogRef = useRef<HTMLDialogElement>(null);

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
  }, [match, mode, offset, pageSize, provider, refresh, search, searchFields]);

  useEffect(() => {
    const dialog = customJobDialogRef.current;
    if (customJobOpen && dialog && !dialog.open) {
      dialog.showModal();
    }
  }, [customJobOpen]);

  function toggleSearchField(field: string, checked: boolean) {
    setOffset(0);
    setSearchFields((current) => checked ? [...current, field] : current.filter((value) => value !== field));
  }

  const matchableUnmatchedJobs = jobs.filter((job) => !job.hasMatch && job.bodyText.trim());
  const queueableJobs = jobsReadyToQueue(matchableUnmatchedJobs, queuedJobIDs);
  const queuedUnmatchedJobs = matchableUnmatchedJobs.length - queueableJobs.length;

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

  async function submitCustomJob(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const sourceURL = customJobURL.trim();
    if (!sourceURL || creatingCustomJob) {
      return;
    }
    setCreatingCustomJob(true);
    try {
      await createCustomJob({ sourceURL });
      customJobDialogRef.current?.close();
      setCustomJobURL("");
      setProvider("");
      setMatch("all");
      setSearch("");
      setOffset(0);
      setRefresh((current) => current + 1);
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not add job");
    } finally {
      setCreatingCustomJob(false);
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
          {mode === "jobs" && <button type="button" className="add-job-action" onClick={() => setCustomJobOpen(true)}>Add job</button>}
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
            <label className="browse-page-size" htmlFor="browse-page-size">Rows per page
              <select id="browse-page-size" value={pageSize} onChange={(event) => { setPageSize(Number(event.target.value)); setOffset(0); }}>
                {pageSizeOptions.map((size) => <option key={size} value={size}>{size}</option>)}
              </select>
            </label>
            <button type="button" className="secondary-action" disabled={offset === 0} onClick={() => setOffset((current) => Math.max(0, current - pageSize))}>Previous</button>
            <button type="button" className="secondary-action" disabled={offset + jobs.length >= totalJobs} onClick={() => setOffset((current) => current + pageSize)}>Next</button>
          </div>
        </footer>
      )}
      {!loading && !error && mode === "jobs" && jobs.length === 0 && <p className="empty browse-empty">No roles match this search.</p>}
      {!loading && !error && mode === "companies" && companies.length === 0 && <p className="empty browse-empty">No companies match this search.</p>}
      <dialog className="custom-job-dialog" ref={customJobDialogRef} onClose={() => setCustomJobOpen(false)}>
        <form onSubmit={(event) => void submitCustomJob(event)}>
          <header>
            <p className="eyebrow">Custom job</p>
            <h2>Add a job posting</h2>
            <p>Paste a posting URL. We will fetch its content, convert it to Markdown, and extract the job details.</p>
          </header>
          <label htmlFor="custom-job-url">Posting URL
            <input id="custom-job-url" type="url" autoFocus required value={customJobURL} onChange={(event) => setCustomJobURL(event.target.value)} placeholder="https://careers.example.com/jobs/..." />
          </label>
          <div className="custom-job-actions">
            <button type="button" className="secondary-action" disabled={creatingCustomJob} onClick={() => customJobDialogRef.current?.close()}>Cancel</button>
            <button type="submit" className="add-job-action" disabled={creatingCustomJob || !customJobURL.trim()}>{creatingCustomJob ? "Importing..." : "Import job"}</button>
          </div>
        </form>
      </dialog>
    </section>
  );
}
