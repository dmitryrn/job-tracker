import { useEffect, useRef, useState } from "react";
import type { Database } from "sql.js";
import { queryBrowseCompanies, queryBrowseJobs, queryBrowseProviders, type BrowseCompany, type BrowseJob } from "./database";

export type BrowseMode = "jobs" | "companies";

type BrowseViewProps = {
  database: Database;
  mode: BrowseMode;
  onModeChange: (mode: BrowseMode) => void;
};

function formatDate(value: string) {
  if (!value) {
    return "Recently posted";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "Recently posted";
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

export default function BrowseView({ database, mode, onModeChange }: BrowseViewProps) {
  const [search, setSearch] = useState("");
  const [provider, setProvider] = useState("");
  const [providers, setProviders] = useState<string[]>([]);
  const [jobs, setJobs] = useState<BrowseJob[]>([]);
  const [companies, setCompanies] = useState<BrowseCompany[]>([]);
  const [error, setError] = useState("");
  const [selectedJob, setSelectedJob] = useState<BrowseJob>();
  const jobDialog = useRef<HTMLDialogElement>(null);

  useEffect(() => {
    try {
      setProviders(queryBrowseProviders(database));
      if (mode === "jobs") {
        setJobs(queryBrowseJobs(database, search, provider));
      } else {
        setCompanies(queryBrowseCompanies(database, search));
      }
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not load browse data");
    }
  }, [database, mode, provider, search]);

  useEffect(() => {
    const dialog = jobDialog.current;
    if (selectedJob && dialog && !dialog.open) {
      dialog.showModal();
    }
  }, [selectedJob]);

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
        {search && <button type="button" onClick={() => setSearch("")}>Clear</button>}
      </form>

      {error && <p className="query-error">{error}</p>}
      {mode === "jobs" ? (
        <div className="job-grid">
          {jobs.map((job) => <JobCard key={job.id} job={job} onOpen={() => setSelectedJob(job)} />)}
        </div>
      ) : (
        <div className="company-grid">
          {companies.map((company) => <CompanyCard key={company.id} company={company} />)}
        </div>
      )}
      {!error && mode === "jobs" && jobs.length === 0 && <p className="empty browse-empty">No roles match this search.</p>}
      {!error && mode === "companies" && companies.length === 0 && <p className="empty browse-empty">No companies match this search.</p>}

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
          <a className="primary-action" href={selectedJob.sourceURL} target="_blank" rel="noreferrer">Open original listing</a>
        </dialog>
      )}
    </section>
  );
}
