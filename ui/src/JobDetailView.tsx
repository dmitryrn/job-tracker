import { useEffect, useState } from "react";
import { deleteJob, fetchJobMatch, type BrowseJob, type JobMatch } from "./api";

type JobDetailViewProps = {
  job: BrowseJob;
  onBack: () => void;
  onDeleted: () => void;
};

function plainText(value: string) {
  if (!/<[a-z][\s\S]*>/i.test(value)) {
    return value;
  }
  return new DOMParser().parseFromString(value, "text/html").body.innerText;
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

export default function JobDetailView({ job, onBack, onDeleted }: JobDetailViewProps) {
  const [tab, setTab] = useState<"post" | "match">("post");
  const [match, setMatch] = useState<JobMatch | null>();
  const [error, setError] = useState("");
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    async function loadMatch() {
      try {
        const result = await fetchJobMatch(job.id, controller.signal);
        if (!controller.signal.aborted) {
          setMatch(result.match);
          setError("");
        }
      } catch (reason) {
        if (!(reason instanceof DOMException && reason.name === "AbortError")) {
          setError(reason instanceof Error ? reason.message : "Could not load match");
        }
      }
    }
    void loadMatch();
    return () => controller.abort();
  }, [job.id]);

  async function removeJob() {
    if (!window.confirm(`Delete ${job.title} from the database?`)) {
      return;
    }
    setDeleting(true);
    try {
      await deleteJob(job.id);
      onDeleted();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not delete job");
      setDeleting(false);
    }
  }

  return (
    <section className="job-page">
      <button type="button" className="back-link" onClick={onBack}>Back to jobs</button>
      <header className="job-page-header">
        <div>
          <p className="eyebrow">{job.source}</p>
          <h1>{job.title}</h1>
          <p>{job.company || "Company not listed"}</p>
        </div>
        <a className="primary-action" href={job.sourceURL} target="_blank" rel="noreferrer">Open original listing</a>
      </header>
      <div className="job-page-meta"><span>{job.location || "Location flexible"}</span><span>{job.employmentType || "Role type not listed"}</span><span>{formatSalary(job)}</span></div>
      <nav className="detail-tabs" aria-label="Job details">
        <button className={tab === "post" ? "detail-tab active" : "detail-tab"} onClick={() => setTab("post")}>Job post</button>
        <button className={tab === "match" ? "detail-tab active" : "detail-tab"} onClick={() => setTab("match")}>Match</button>
      </nav>
      {error && <p className="query-error">{error}</p>}
      {tab === "post" ? <p className="job-post">{plainText(job.bodyText)}</p> : (
        <section className="match-panel">
          <p className="eyebrow">Current match</p>
          {match === undefined ? <p>Loading match...</p> : match === null ? <p>This job is waiting for the background matcher. Save your profile first if you have not already.</p> : <p>{match.content}</p>}
        </section>
      )}
      <div className="job-page-actions"><button type="button" className="danger-action" disabled={deleting} onClick={() => void removeJob()}>{deleting ? "Deleting..." : "Delete from database"}</button></div>
    </section>
  );
}
