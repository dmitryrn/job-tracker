import { useEffect, useRef, useState } from "react";
import { deleteJob, fetchCompanies, fetchJobs, fetchProviders, type BrowseCompany, type BrowseJob } from "./api";

export type BrowseMode = "jobs" | "companies";

type BrowseViewProps = {
  mode: BrowseMode;
  onModeChange: (mode: BrowseMode) => void;
};

const searchFieldOptions = [
  { value: "title", label: "Title" },
  { value: "company", label: "Company" },
  { value: "location", label: "Location" },
  { value: "body", label: "Description" },
];

export function formatDate(value: string, now = new Date()) {
  if (!value) {
    return "Recently posted";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "Recently posted";
  }
  const daysAgo = Math.floor((now.getTime() - date.getTime()) / (24 * 60 * 60 * 1000));
  if (daysAgo <= 0) {
    return "Today";
  }
  if (daysAgo <= 30) {
    return `${daysAgo}d ago`;
  }
  return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", year: "numeric" }).format(date);
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

function JobCard({ job, onOpen }: { job: BrowseJob; onOpen: () => void }) {
  return (
    <article className="job-card">
      <button className="job-card-button" onClick={onOpen}>
        <div className="job-card-topline">
          <span className="source-label">{job.source}</span>
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

export default function BrowseView({ mode, onModeChange }: BrowseViewProps) {
  const [search, setSearch] = useState("");
  const [provider, setProvider] = useState("");
  const [searchFields, setSearchFields] = useState(searchFieldOptions.map(({ value }) => value));
  const [providers, setProviders] = useState<string[]>([]);
  const [jobs, setJobs] = useState<BrowseJob[]>([]);
  const [companies, setCompanies] = useState<BrowseCompany[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [deleting, setDeleting] = useState(false);
  const [selectedJob, setSelectedJob] = useState<BrowseJob>();
  const jobDialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      setLoading(true);
      try {
        if (mode === "jobs") {
          const [jobResult, providerResult] = await Promise.all([
            fetchJobs(search, provider, searchFields, controller.signal),
            fetchProviders(controller.signal),
          ]);
          setJobs(jobResult.jobs);
          setProviders(providerResult.providers);
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
  }, [mode, provider, search, searchFields]);

  useEffect(() => {
    const dialog = jobDialog.current;
    if (selectedJob && dialog && !dialog.open) {
      dialog.showModal();
    }
  }, [selectedJob]);

  function toggleSearchField(field: string, checked: boolean) {
    setSearchFields((current) => checked ? [...current, field] : current.filter((value) => value !== field));
  }

  async function removeSelectedJob() {
    if (!selectedJob || !window.confirm(`Delete ${selectedJob.title} from the database?`)) {
      return;
    }
    setDeleting(true);
    try {
      await deleteJob(selectedJob.id);
      setJobs((current) => current.filter((job) => job.id !== selectedJob.id));
      setSelectedJob(undefined);
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not delete job");
    } finally {
      setDeleting(false);
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
        <div className="browse-stat">
          <strong>{mode === "jobs" ? jobs.length : companies.length}</strong>
          <span>{mode === "jobs" ? "roles in view" : "companies in view"}</span>
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
          onChange={(event) => setSearch(event.target.value)}
          placeholder={mode === "jobs" ? "Search titles, companies, or locations" : "Search company names"}
        />
        {mode === "jobs" && (
          <>
            <label htmlFor="provider">Provider</label>
            <select id="provider" value={provider} onChange={(event) => setProvider(event.target.value)}>
              <option value="">All providers</option>
              {providers.map((value) => <option key={value} value={value}>{formatProvider(value)}</option>)}
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
        {search && <button type="button" onClick={() => setSearch("")}>Clear</button>}
      </form>

      {error && <p className="query-error">{error}</p>}
      {loading && <p className="browse-loading">Loading...</p>}
      {mode === "jobs" ? (
        <div className="job-grid">
          {jobs.map((job) => <JobCard key={job.id} job={job} onOpen={() => setSelectedJob(job)} />)}
        </div>
      ) : (
        <div className="company-grid">
          {companies.map((company) => <CompanyCard key={company.id} company={company} />)}
        </div>
      )}
      {!loading && !error && mode === "jobs" && jobs.length === 0 && <p className="empty browse-empty">No roles match this search.</p>}
      {!loading && !error && mode === "companies" && companies.length === 0 && <p className="empty browse-empty">No companies match this search.</p>}

      {selectedJob && (
        <dialog
          aria-labelledby="job-dialog-title"
          className="job-dialog"
          onCancel={() => setSelectedJob(undefined)}
          onClick={(event) => {
            if (event.target === event.currentTarget) {
              setSelectedJob(undefined);
            }
          }}
          ref={jobDialog}
        >
          <div className="job-dialog-header">
            <div>
              <p className="eyebrow">{selectedJob.source} / {formatDate(selectedJob.postedAt)}</p>
              <h2 id="job-dialog-title">{selectedJob.title}</h2>
              <p>{selectedJob.company || "Company not listed"}</p>
            </div>
            <button type="button" className="dialog-close" onClick={() => setSelectedJob(undefined)}>Close</button>
          </div>
          <div className="job-dialog-meta">
            <span>{selectedJob.location || "Location flexible"}</span>
            <span>{selectedJob.employmentType || "Role type not listed"}</span>
            <span>{formatSalary(selectedJob)}</span>
          </div>
          <p className="job-description">{plainText(selectedJob.bodyText)}</p>
            <div className="job-dialog-actions">
              <a className="primary-action" href={selectedJob.sourceURL} target="_blank" rel="noreferrer">Open original listing</a>
              <button type="button" className="danger-action" disabled={deleting} onClick={() => void removeSelectedJob()}>{deleting ? "Deleting..." : "Delete from database"}</button>
            </div>
        </dialog>
      )}
    </section>
  );
}
