import { useEffect, useState } from "react";
import { deleteJob, fetchJobMatch, queueJobMatch, type BrowseJob, type JobAnalysis, type JobMatch, type JobMatchAssessment } from "./api";

type JobDetailViewProps = {
  job: BrowseJob;
  tab: "post" | "match";
  onTabChange: (tab: "post" | "match") => void;
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

function humanize(value: string) {
  return value.replaceAll("_", " ").replaceAll("-", " ");
}

function formatAnalysisDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(date);
}

function JobAnalysisPanel({ record }: { record: JobAnalysis }) {
  const draft = record.analysis;
  return (
    <section className="analysis-panel">
      <div className="analysis-panel-header">
        <div>
          <p className="eyebrow">Job analysis</p>
          <h2>{humanize(draft.role.family)} <span>{humanize(draft.role.seniority)}</span></h2>
        </div>
        <p>{record.model}<br />{formatAnalysisDate(record.analyzedAt)}</p>
      </div>
      {draft.constraints.length > 0 && (
        <section className="analysis-section">
          <h3>Role constraints</h3>
          <div className="analysis-grid">
            {draft.constraints.map((constraint) => <article className="analysis-card" key={`${constraint.kind}-${constraint.quote}`}><p>{humanize(constraint.kind)} · {constraint.confidence} confidence</p><strong>{constraint.value}</strong><blockquote>{constraint.quote}</blockquote></article>)}
          </div>
        </section>
      )}
      {draft.requirements.length > 0 && (
        <section className="analysis-section">
          <h3>Requirements</h3>
          <div className="analysis-grid">
            {draft.requirements.map((requirement) => <article className="analysis-card" key={requirement.id}><p>{humanize(requirement.kind)} · {requirement.screeningRisk} screening risk</p><strong>{humanize(requirement.concept)}{requirement.minimumYears === null ? "" : ` · ${requirement.minimumYears}+ years`}</strong><blockquote>{requirement.quote}</blockquote></article>)}
          </div>
        </section>
      )}
      {draft.responsibilities.length > 0 && (
        <section className="analysis-section">
          <h3>Responsibilities</h3>
          <ul className="analysis-evidence-list">{draft.responsibilities.map((item) => <li key={`${item.concept}-${item.quote}`}><strong>{humanize(item.concept)}</strong><span>{item.quote}</span></li>)}</ul>
        </section>
      )}
      {draft.preferences.length > 0 && (
        <section className="analysis-section">
          <h3>Preferences</h3>
          <ul className="analysis-evidence-list">{draft.preferences.map((item) => <li key={`${item.concept}-${item.quote}`}><strong>{humanize(item.concept)}</strong><span>{item.quote}</span></li>)}</ul>
        </section>
      )}
      {draft.unknowns.length > 0 && (
        <section className="analysis-section">
          <h3>Not established</h3>
          <ul className="analysis-unknowns">{draft.unknowns.map((unknown) => <li key={unknown}>{unknown}</li>)}</ul>
        </section>
      )}
    </section>
  );
}

function JobMatchAssessmentPanel({ assessment }: { assessment: JobMatchAssessment }) {
  return (
    <>
      <div className="match-score"><strong>{assessment.score}</strong><span>/100</span><p>{assessment.label}</p></div>
      <p>{assessment.summary}</p>
      {assessment.strengths.length > 0 && <section><h3>Strengths</h3><ul>{assessment.strengths.map((strength) => <li key={strength}>{strength}</li>)}</ul></section>}
      {assessment.gaps.length > 0 && <section><h3>Gaps</h3><ul>{assessment.gaps.map((gap) => <li key={gap}>{gap}</li>)}</ul></section>}
      {assessment.questions.length > 0 && <section><h3>Questions to resolve</h3><ul>{assessment.questions.map((question) => <li key={question}>{question}</li>)}</ul></section>}
      {assessment.applicationAngle && <section><h3>Application angle</h3><p>{assessment.applicationAngle}</p></section>}
    </>
  );
}

export default function JobDetailView({ job, tab, onTabChange, onBack, onDeleted }: JobDetailViewProps) {
  const [match, setMatch] = useState<JobMatch | null>();
  const [analysis, setAnalysis] = useState<JobAnalysis | null>();
  const [error, setError] = useState("");
  const [deleting, setDeleting] = useState(false);
  const [queueing, setQueueing] = useState(false);
  const [queued, setQueued] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    async function loadMatch() {
      setMatch(undefined);
      setAnalysis(undefined);
      try {
        const result = await fetchJobMatch(job.id, controller.signal);
        if (!controller.signal.aborted) {
          setMatch(result.match);
		  setAnalysis(result.analysis);
          setError("");
          setQueued(false);
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

  async function requestMatch(redo: boolean) {
    setQueueing(true);
    try {
      await queueJobMatch(job.id, redo);
      setMatch(null);
      setQueued(true);
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not queue match");
    } finally {
      setQueueing(false);
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
        <button className={tab === "post" ? "detail-tab active" : "detail-tab"} onClick={() => onTabChange("post")}>Job post</button>
        <button className={tab === "match" ? "detail-tab active" : "detail-tab"} onClick={() => onTabChange("match")}>Match</button>
      </nav>
      {error && <p className="query-error">{error}</p>}
      {tab === "post" ? <p className="job-post">{plainText(job.bodyText)}</p> : (
        <>
          <section className="match-panel">
            <p className="eyebrow">Current match</p>
            {match === undefined ? <p>Loading match...</p> : match === null ? <p>{queued ? "Match request queued." : "No match yet."}</p> : match.assessment ? <JobMatchAssessmentPanel assessment={match.assessment} /> : <p>{match.content}</p>}
            {match !== undefined && <button type="button" className="secondary-action" disabled={queueing} onClick={() => void requestMatch(match !== null)}>{queueing ? "Queueing..." : match === null ? "Create match" : "Redo match"}</button>}
          </section>
          {analysis === undefined ? <p className="analysis-loading">Loading job analysis...</p> : analysis && <JobAnalysisPanel record={analysis} />}
        </>
      )}
      <div className="job-page-actions"><button type="button" className="danger-action" disabled={deleting} onClick={() => void removeJob()}>{deleting ? "Deleting..." : "Delete from database"}</button></div>
    </section>
  );
}
