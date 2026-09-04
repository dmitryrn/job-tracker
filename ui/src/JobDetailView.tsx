import { type FormEvent, useEffect, useRef, useState } from "react";
import { deleteJob, fetchJobMatch, fetchJobMatchApplicationResume, fetchJobMatchChat, queueJobMatch, revertJobMatchChat, sendJobMatchChatMessage, type ApplicationResume, type ApplicationResumeAgentEvent, type BrowseJob, type JobAnalysis, type JobMatch, type JobMatchAssessment, type JobMatchChatMessage, type Resume } from "./api";

type JobDetailViewProps = {
  job: BrowseJob;
	tab: "post" | "match" | "chat";
	onTabChange: (tab: "post" | "match" | "chat") => void;
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
    return "Not listed";
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

function chatRequestID() {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("").replace(/^(.{8})(.{4})(.{4})(.{4})(.{12})$/, "$1-$2-$3-$4-$5");
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

function JobMatchChatPanel({ jobID, match }: { jobID: number; match: JobMatch | null | undefined }) {
  const [messages, setMessages] = useState<JobMatchChatMessage[]>();
  const [applicationResume, setApplicationResume] = useState<ApplicationResume | null>();
  const [agentEvents, setAgentEvents] = useState<ApplicationResumeAgentEvent[]>([]);
  const [draft, setDraft] = useState("");
	const [requestID, setRequestID] = useState<string>();
	const [sending, setSending] = useState(false);
	const [reverting, setReverting] = useState(false);
	const [error, setError] = useState("");
	const sendController = useRef<AbortController | undefined>(undefined);

	useEffect(() => () => sendController.current?.abort(), []);

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      setMessages(undefined);
      try {
        const [result, application] = await Promise.all([
          fetchJobMatchChat(jobID, controller.signal),
          fetchJobMatchApplicationResume(jobID, controller.signal),
        ]);
        if (!controller.signal.aborted) {
          setMessages(result.messages);
          setApplicationResume(application.resume);
          setAgentEvents(application.events);
          setError("");
        }
      } catch (reason) {
        if (!(reason instanceof DOMException && reason.name === "AbortError")) {
          setError(reason instanceof Error ? reason.message : "Could not load chat");
        }
      }
    }
    void load();
    return () => controller.abort();
  }, [jobID]);

  async function send(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const content = draft.trim();
    if (!content || sending) {
      return;
		}
		setSending(true);
		const controller = new AbortController();
		sendController.current = controller;
		try {
			const currentRequestID = requestID ?? chatRequestID();
			setRequestID(currentRequestID);
			await sendJobMatchChatMessage(jobID, content, currentRequestID, controller.signal);
			setDraft("");
			setRequestID(undefined);
			setError("");
		} catch (reason) {
			setError(controller.signal.aborted ? "Response stopped." : reason instanceof Error ? reason.message : "Could not send chat message");
		} finally {
			if (sendController.current === controller) {
				sendController.current = undefined;
			}
			try {
        const controller = new AbortController();
        const [result, application] = await Promise.all([
          fetchJobMatchChat(jobID, controller.signal),
          fetchJobMatchApplicationResume(jobID, controller.signal),
        ]);
        setMessages(result.messages);
        setApplicationResume(application.resume);
        setAgentEvents(application.events);
      } catch (reason) {
        setError(reason instanceof Error ? reason.message : "Could not load chat");
      }
      setSending(false);
		}
	}

	function stop() {
		sendController.current?.abort();
	}

  async function revert(message: JobMatchChatMessage) {
    if (sending || reverting) {
      return;
    }
    setReverting(true);
    try {
      await revertJobMatchChat(jobID, message.id);
      setMessages((current) => current?.filter((item) => item.id < message.id));
      setApplicationResume((current) => current?.rootMessageId === message.id ? null : current ? { ...current, revisions: current.revisions.filter((revision) => revision.triggerMessageId < message.id) } : current);
      setAgentEvents((current) => current.filter((event) => event.triggerMessageId < message.id));
      setDraft(message.content);
      setRequestID(undefined);
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not revert chat");
    } finally {
      setReverting(false);
    }
  }

  if (match === undefined) {
    return <p className="analysis-loading">Loading match...</p>;
  }
  if (match === null) {
    return <section className="match-chat"><p>Create a match before starting a chat about this role.</p></section>;
  }

  return <section className="match-chat">
    <header className="match-chat-header"><div><p className="eyebrow">Match chat</p><h2>Discuss this opportunity</h2></div><p>Uses the job post, current match, profile, and base resume.</p></header>
    {error && <p className="query-error">{error}</p>}
    {messages === undefined ? <p className="analysis-loading">Loading chat...</p> : <div className="match-chat-messages" aria-live="polite">
      {messages.length === 0 ? <p className="match-chat-empty">Ask about fit, gaps, interview preparation, or how to tailor your application.</p> : messages.map((message) => <article className={`match-chat-message ${message.role}`} key={message.id}>
        <header><p>{message.role === "user" ? "You" : "AI"}</p>{message.role === "user" && <button type="button" className="match-chat-revert" onClick={() => void revert(message)} disabled={sending || reverting} aria-label="Revert to this message" title="Revert to this message"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 7-5 5 5 5M4 12h9a6 6 0 0 1 6 6" /></svg></button>}</header>
        <div>{message.content}</div>
      </article>)}
    </div>}
    {applicationResume && <ApplicationResumeActivity applicationResume={applicationResume} events={agentEvents} />}
	<form className="match-chat-compose" onSubmit={(event) => void send(event)}>
		<label>Message<textarea value={draft} onChange={(event) => { setDraft(event.target.value); setRequestID(undefined); }} disabled={sending || reverting} placeholder="Ask about this job and your fit..." rows={3} /></label>
		<div className="match-chat-actions"><button className="primary-action" type="submit" disabled={sending || reverting || !draft.trim()}>{sending ? "Thinking..." : "Send"}</button>{sending && <button className="secondary-action" type="button" onClick={stop}>Stop</button>}</div>
	</form>
  </section>;
}

function ApplicationResumeActivity({ applicationResume, events }: { applicationResume: ApplicationResume; events: ApplicationResumeAgentEvent[] }) {
  return <section className="application-resume-activity">
    <header><p className="eyebrow">Application resume</p><h3>Revision history</h3></header>
    {applicationResume.revisions.map((revision, index) => <article className="application-resume-revision" key={revision.id}>
      <strong>{revision.revisionNumber === 0 ? "Base snapshot" : `Revision ${revision.revisionNumber}`}</strong>
      <p>{revision.summary}</p>
      {index > 0 && <ul>{resumeDiff(applicationResume.revisions[index - 1].resume, revision.resume).map((change) => <li key={change}>{change}</li>)}</ul>}
    </article>)}
    {events.length > 0 && <details className="application-resume-events"><summary>Resume edit activity</summary><ol>{events.map((event) => <li key={event.id}>{event.detail}</li>)}</ol></details>}
  </section>;
}

function resumeDiff(previous: Resume, next: Resume) {
  const changes: string[] = [];
  if (previous.headline !== next.headline) changes.push(`Headline: ${previous.headline || "(empty)"} -> ${next.headline || "(empty)"}`);
  const previousSummary = new Map(previous.summaryParagraphs.map((item) => [item.id, item.content]));
  for (const item of next.summaryParagraphs) if (previousSummary.get(item.id) !== item.content) changes.push(previousSummary.has(item.id) ? `Summary: ${previousSummary.get(item.id)} -> ${item.content}` : `Added summary: ${item.content}`);
  const previousSkills = new Map(previous.skills.map((item) => [item.id, item.name]));
  for (const item of next.skills) if (previousSkills.get(item.id) !== item.name) changes.push(previousSkills.has(item.id) ? `Skill: ${previousSkills.get(item.id)} -> ${item.name}` : `Added skill: ${item.name}`);
  for (const item of previous.skills) if (!next.skills.some((current) => current.id === item.id)) changes.push(`Removed skill: ${item.name}`);
  const previousBullets = new Map(previous.experience.flatMap((entry) => entry.bullets.map((bullet) => [bullet.id, bullet.content])));
  for (const bullet of next.experience.flatMap((entry) => entry.bullets)) if (previousBullets.get(bullet.id) !== bullet.content) changes.push(previousBullets.has(bullet.id) ? `Experience bullet: ${previousBullets.get(bullet.id)} -> ${bullet.content}` : `Added experience bullet: ${bullet.content}`);
  const previousCompetencyBullets = new Map(previous.competencies.flatMap((entry) => entry.bullets.map((bullet) => [bullet.id, bullet.content])));
  for (const bullet of next.competencies.flatMap((entry) => entry.bullets)) if (previousCompetencyBullets.get(bullet.id) !== bullet.content) changes.push(previousCompetencyBullets.has(bullet.id) ? `Competency bullet: ${previousCompetencyBullets.get(bullet.id)} -> ${bullet.content}` : `Added competency bullet: ${bullet.content}`);
  return changes.length > 0 ? changes : ["Updated application resume"];
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
      <dl className="job-page-meta">
        <div><dt>Location</dt><dd>{job.location || "Location flexible"}</dd></div>
        <div><dt>Employment type</dt><dd>{job.employmentType || "Not listed"}</dd></div>
        <div><dt>Compensation</dt><dd>{formatSalary(job)}</dd></div>
      </dl>
      <nav className="detail-tabs" aria-label="Job details">
        <button className={tab === "post" ? "detail-tab active" : "detail-tab"} onClick={() => onTabChange("post")}>Job post</button>
        <button className={tab === "match" ? "detail-tab active" : "detail-tab"} onClick={() => onTabChange("match")}>Match</button>
        <button className={tab === "chat" ? "detail-tab active" : "detail-tab"} onClick={() => onTabChange("chat")}>Chat</button>
      </nav>
      {error && <p className="query-error">{error}</p>}
      {tab === "post" ? <p className="job-post">{plainText(job.bodyText)}</p> : tab === "chat" ? <JobMatchChatPanel jobID={job.id} match={match} /> : (
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
