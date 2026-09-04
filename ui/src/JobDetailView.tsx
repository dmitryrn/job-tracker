import { type FormEvent, useEffect, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { deleteJob, fetchJobMatch, fetchJobMatchChat, jobMatchChatEventsURL, queueJobMatch, revertJobMatchChat, sendJobMatchChatMessage, stopJobMatchChat, type BrowseJob, type JobAnalysis, type JobMatch, type JobMatchAssessment, type JobMatchChatItem, type Resume } from "./api";
import JSONTree from "./JSONTree";

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
  const [items, setItems] = useState<JobMatchChatItem[]>();
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [reverting, setReverting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    async function load(reset = false) {
      if (reset) setItems(undefined);
      try {
        const result = await fetchJobMatchChat(jobID, controller.signal);
        if (!controller.signal.aborted) {
          setItems(result.items);
          setError("");
        }
      } catch (reason) {
        if (!(reason instanceof DOMException && reason.name === "AbortError")) {
          setError(reason instanceof Error ? reason.message : "Could not load chat");
        }
      }
    }
    void load(true);
    const events = new EventSource(jobMatchChatEventsURL(jobID));
    events.addEventListener("item", (event) => {
      const item = JSON.parse((event as MessageEvent<string>).data) as JobMatchChatItem;
      setItems((current) => {
        if (!current) return current;
        if (current.some((existing) => existing.sequence === item.sequence)) return current;
        return [...current, item].sort((left, right) => left.sequence - right.sequence);
      });
    });
    events.addEventListener("reset", () => void load());
    events.onerror = () => { /* EventSource reconnects; the server repairs item gaps on reconnect. */ };
    return () => { controller.abort(); events.close(); };
  }, [jobID]);

  const messages = (items ?? []).flatMap((item) => item.type === "user_message" || item.type === "assistant_message" ? [{ item, content: contentOf(item) }] : []);
  const activeRequestID = activeRequest(items ?? []);
  const unanswered = messages.at(-1)?.item.type === "user_message";
  const blocked = sending || reverting || unanswered;

  async function send(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const content = draft.trim();
    if (!content || blocked) {
      return;
    }
    setSending(true);
    try {
      await sendJobMatchChatMessage(jobID, content, chatRequestID());
      setDraft("");
      setError("");
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not send chat message");
    } finally {
      setSending(false);
    }
  }

  async function stop() {
    if (!activeRequestID) return;
    try { await stopJobMatchChat(jobID, activeRequestID); } catch (reason) { setError(reason instanceof Error ? reason.message : "Could not stop chat"); }
  }

  async function revert(message: JobMatchChatItem, content: string) {
    if (reverting) {
      return;
    }
    setReverting(true);
    try {
      await revertJobMatchChat(jobID, message.sequence);
      setItems((current) => current?.filter((item) => item.sequence < message.sequence));
      setDraft(content);
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
    {error && <p className="query-error">{error}</p>}
    {items === undefined ? <p className="analysis-loading">Loading chat...</p> : <ChatTimeline items={items} onRevert={revert} reverting={reverting} />}
    <form className="match-chat-compose" onSubmit={(event) => void send(event)}>
      <label>Message<textarea value={draft} onChange={(event) => setDraft(event.target.value)} disabled={blocked} placeholder="Ask about this job and your fit..." rows={3} /></label>
      <div className="match-chat-actions"><button className="primary-action" type="submit" disabled={blocked || !draft.trim()}>{sending || activeRequestID ? "Thinking..." : "Send"}</button>{activeRequestID && <button className="secondary-action" type="button" onClick={() => void stop()}>Stop</button>}</div>
    </form>
  </section>;
}

type ResumeRevision = { sequence: number; revision: number; resume: Resume; summary: string };

function contentOf(item: JobMatchChatItem) {
  return typeof item.payload === "object" && item.payload !== null && "content" in item.payload && typeof item.payload.content === "string" ? item.payload.content : "";
}

function resumeRevisions(items: JobMatchChatItem[]): ResumeRevision[] {
  return items.flatMap((item) => {
    const value = item.payload as { status?: string; revision?: number; resume?: Resume; summary?: string };
    if ((item.type === "resume_revision" || item.type === "tool_result") && (item.type !== "tool_result" || value.status === "accepted") && value.resume && typeof value.revision === "number") return [{ sequence: item.sequence, revision: value.revision, resume: value.resume, summary: value.summary || "Application resume updated" }];
    return [];
  });
}

function activeRequest(items: JobMatchChatItem[]) {
  const user = [...items].reverse().find((item) => item.type === "user_message");
  if (!user?.requestId) return undefined;
  const terminal = items.some((item) => item.requestId === user.requestId && ["turn_completed", "turn_stopped", "turn_halted"].includes(item.type));
  return terminal ? undefined : user.requestId;
}

function ChatTimeline({ items, onRevert, reverting }: { items: JobMatchChatItem[]; onRevert: (item: JobMatchChatItem, content: string) => Promise<void>; reverting: boolean }) {
  const revisions = resumeRevisions(items);
  const visible = items.filter((item) => ["user_message", "assistant_message", "assistant_reasoning", "assistant_tool_call", "tool_result", "patch_retrying", "retry_limit_reached", "turn_error", "turn_stopped"].includes(item.type));
  return <div className="match-chat-messages" aria-live="polite">
    {visible.length === 0 && <p className="match-chat-empty">Ask about fit, gaps, interview preparation, or how to tailor your application.</p>}
    {visible.map((item) => {
      if (item.type === "user_message" || item.type === "assistant_message") {
        const content = contentOf(item);
        if (item.type === "user_message") return <div className="match-chat-user-turn" key={item.sequence}>
          <article className="match-chat-message user"><div>{content}</div></article>
          <button type="button" className="match-chat-revert" onClick={() => void onRevert(item, content)} disabled={reverting} aria-label="Revert to this message" title="Revert to this message"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 7-5 5 5 5M4 12h9a6 6 0 0 1 6 6" /></svg></button>
        </div>;
        return <article className="match-chat-message assistant" key={item.sequence}><div className="match-chat-markdown"><ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml>{content}</ReactMarkdown></div></article>;
      }
      if (item.type === "tool_result") {
        const revisionIndex = revisions.findIndex((revision) => revision.sequence === item.sequence);
        if (revisionIndex >= 0) return <ApplicationResumeActivity key={item.sequence} revision={revisions[revisionIndex]} previous={revisions[revisionIndex - 1]} />;
      }
      const toolCall = item.type === "assistant_tool_call";
      const toolName = toolCallName(item);
      return <details className={`application-resume-events${toolCall ? " tool-call" : ""}`} key={item.sequence} open={toolCall ? undefined : true}>
        <summary>{toolCall ? <><svg className="tool-call-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M8 4v5M16 15v5M5 9h6v6H5zM13 9h6v6h-6zM11 12h2" /></svg>{toolName}</> : humanize(item.type)}</summary>
        {!toolCall && <p>{activityDetail(item)}</p>}
        <div className="match-chat-activity-details"><JSONTree value={item.payload} /></div>
      </details>;
    })}
  </div>;
}

function activityDetail(item: JobMatchChatItem) {
	const value = item.payload as { summary?: string; detail?: string; error?: string; status?: string; revision?: number; function?: { name?: string } };
	if (item.type === "assistant_tool_call") return value.function?.name ? `Called ${humanize(value.function.name)}` : "Tool call requested";
	if (item.type === "tool_result" && value.status === "accepted") return value.summary || (value.revision === undefined ? "Resume revision accepted" : `Accepted resume revision ${value.revision}`);
	return value.summary || value.detail || value.error || value.status || "Recorded";
}

function toolCallName(item: JobMatchChatItem) {
  const value = item.payload as { function?: { name?: string } };
  return value.function?.name ? humanize(value.function.name) : "Tool call";
}

function ApplicationResumeActivity({ revision, previous }: { revision: ResumeRevision; previous?: ResumeRevision }) {
  return <section className="application-resume-activity">
    <h3>{revision.revision === 0 ? "Base snapshot" : `Revision ${revision.revision} applied`}</h3>
    <article className="application-resume-revision">
      <p>{revision.summary}</p>
      {previous && <ul>{resumeDiff(previous.resume, revision.resume).map((change) => <li key={change}>{change}</li>)}</ul>}
    </article>
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
