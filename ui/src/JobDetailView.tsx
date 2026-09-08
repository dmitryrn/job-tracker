import { type FormEvent, useEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { deleteJob, fetchJobMatch, fetchJobMatchChat, jobApplicationResumePDFURL, jobMatchChatEventsURL, queueJobMatch, rejectJob, revertJobMatchChat, sendJobMatchChatMessage, stopJobMatchChat, type BrowseJob, type JobAnalysis, type JobMatch, type JobMatchAssessment, type JobMatchChatItem, type Resume } from "./api";
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

function formatTimestamp(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(date);
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
    const viewport = window.visualViewport;
    if (!viewport) return;
    function updateComposerOffset() {
      if (!viewport) return;
      const keyboardHeight = Math.max(0, window.innerHeight - viewport.height - viewport.offsetTop);
      document.documentElement.style.setProperty("--chat-compose-keyboard-offset", `${keyboardHeight}px`);
    }
    updateComposerOffset();
    viewport.addEventListener("resize", updateComposerOffset);
    viewport.addEventListener("scroll", updateComposerOffset);
    return () => {
      viewport.removeEventListener("resize", updateComposerOffset);
      viewport.removeEventListener("scroll", updateComposerOffset);
      document.documentElement.style.removeProperty("--chat-compose-keyboard-offset");
    };
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    async function load(reset = false) {
      if (reset) setItems(undefined);
      try {
        const result = await fetchJobMatchChat(jobID, controller.signal);
        if (!controller.signal.aborted) {
          setItems((current) => mergeChatItems(current, result.items));
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
      setItems((current) => mergeChatItems(current, [item]));
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
      const result = await sendJobMatchChatMessage(jobID, content, chatRequestID());
      setItems((current) => mergeChatItems(current, [result.item]));
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

  function keepComposerClear() {
    const distanceFromBottom = document.documentElement.scrollHeight - (window.scrollY + window.innerHeight);
    const atBottom = distanceFromBottom <= 80;
    if (!atBottom) return;
    window.setTimeout(() => window.scrollTo(0, document.documentElement.scrollHeight), 160);
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
      <textarea aria-label="Message" value={draft} onChange={(event) => setDraft(event.target.value)} onFocus={keepComposerClear} disabled={blocked} rows={3} />
      <div className="match-chat-actions"><button className={activeRequestID ? "secondary-action" : "primary-action"} type={activeRequestID ? "button" : "submit"} disabled={!activeRequestID && (blocked || !draft.trim())} onClick={activeRequestID ? () => void stop() : undefined} aria-label={activeRequestID ? "Stop response" : "Send message"} title={activeRequestID ? "Stop response" : "Send message"}>
        {activeRequestID ? <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="7" y="7" width="10" height="10" rx="1" /></svg> : <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 3.5 21 12 3 20.5V14l12-2-12-2z" /></svg>}
      </button></div>
    </form>
  </section>;
}

export function mergeChatItems(current: JobMatchChatItem[] | undefined, incoming: JobMatchChatItem[]) {
  const bySequence = new Map((current ?? []).map((item) => [item.sequence, item]));
  for (const item of incoming) bySequence.set(item.sequence, item);
  return [...bySequence.values()].sort((left, right) => left.sequence - right.sequence);
}

type ResumeRevision = { sequence: number; revision: number; resume: Resume };
type ResumeChange = { key: string; label: string; removed?: string; added?: string };

function contentOf(item: JobMatchChatItem) {
  return typeof item.payload === "object" && item.payload !== null && "content" in item.payload && typeof item.payload.content === "string" ? item.payload.content : "";
}

function resumeRevisions(items: JobMatchChatItem[]): ResumeRevision[] {
  return items.flatMap((item) => {
    const value = item.payload as { status?: string; revision?: number; resume?: Resume };
    if ((item.type === "resume_revision" || item.type === "tool_result") && (item.type !== "tool_result" || value.status === "accepted") && value.resume && typeof value.revision === "number") return [{ sequence: item.sequence, revision: value.revision, resume: value.resume }];
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
  if (value.function?.name === "revise_application_resume") return "revised application resume";
  return value.function?.name ? humanize(value.function.name) : "Tool call";
}

function ApplicationResumeActivity({ revision, previous }: { revision: ResumeRevision; previous?: ResumeRevision }) {
  const changes = previous ? resumeDiff(previous.resume, revision.resume) : [];
  const groups = changes.reduce<Array<{ label: string; changes: ResumeChange[] }>>((all, change) => {
    const group = all.at(-1);
    if (group?.label === change.label) group.changes.push(change);
    else all.push({ label: change.label, changes: [change] });
    return all;
  }, []);
  return <section className="application-resume-activity">
    <h3>{revision.revision === 0 ? "Base snapshot" : `Revision ${revision.revision} applied`}</h3>
    <article className="application-resume-revision">
      {groups.length > 0 && <div className="resume-diff" aria-label="Resume changes">
        {groups.map((group) => <section className="resume-diff-group" key={group.label}>
          <h4>{group.label}</h4>
          {group.changes.map((change) => <div className="resume-diff-change" key={change.key}>
            {change.removed !== undefined && <div className="resume-diff-line removed"><span className="resume-diff-prefix" aria-hidden="true">-</span><span>{change.removed || "(empty)"}</span></div>}
            {change.added !== undefined && <div className="resume-diff-line added"><span className="resume-diff-prefix" aria-hidden="true">+</span><span>{change.added || "(empty)"}</span></div>}
          </div>)}
        </section>)}
      </div>}
    </article>
  </section>;
}

function resumeDiff(previous: Resume, next: Resume) {
  const changes: ResumeChange[] = [];
  const appendChanges = (label: string, keyPrefix: string, before: Array<{ id: number; content: string }>, after: Array<{ id: number; content: string }>) => {
    const beforeByID = new Map(before.map((item) => [item.id, item.content]));
    const afterByID = new Map(after.map((item) => [item.id, item.content]));
    for (const item of after) {
      const prior = beforeByID.get(item.id);
      if (prior === undefined) changes.push({ key: `${keyPrefix}-${item.id}`, label, added: item.content });
      else if (prior !== item.content) changes.push({ key: `${keyPrefix}-${item.id}`, label, removed: prior, added: item.content });
    }
    for (const item of before) if (!afterByID.has(item.id)) changes.push({ key: `${keyPrefix}-${item.id}`, label, removed: item.content });
  };

  if (previous.headline !== next.headline) changes.push({ key: "headline", label: "Headline", removed: previous.headline, added: next.headline });
  appendChanges("Summary", "summary", previous.summaryParagraphs, next.summaryParagraphs);
  appendChanges("Skill", "skill", previous.skills.map((item) => ({ id: item.id, content: item.name })), next.skills.map((item) => ({ id: item.id, content: item.name })));
  appendChanges("Experience bullet", "experience-bullet", previous.experience.flatMap((entry) => entry.bullets), next.experience.flatMap((entry) => entry.bullets));
  appendChanges("Competency bullet", "competency-bullet", previous.competencies.flatMap((entry) => entry.bullets), next.competencies.flatMap((entry) => entry.bullets));
  return changes;
}

export default function JobDetailView({ job, tab, onTabChange, onBack, onDeleted }: JobDetailViewProps) {
  const [match, setMatch] = useState<JobMatch | null>();
  const [analysis, setAnalysis] = useState<JobAnalysis | null>();
  const [error, setError] = useState("");
  const [deleting, setDeleting] = useState(false);
  const [rejecting, setRejecting] = useState(false);
  const [actionsOpen, setActionsOpen] = useState(false);
  const [rejectionOpen, setRejectionOpen] = useState(false);
  const [rejectionReason, setRejectionReason] = useState("");
  const [queueing, setQueueing] = useState(false);
  const [queued, setQueued] = useState(false);
  const actionsMenuRef = useRef<HTMLDetailsElement>(null);
  const rejectionDialogRef = useRef<HTMLDialogElement>(null);
  const postedAt = formatTimestamp(job.postedAt);
  const matchedAt = match ? formatTimestamp(match.createdAt) : "";

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

  useEffect(() => {
    if (!actionsOpen) return;
    function closeActions(event: PointerEvent) {
      if (!actionsMenuRef.current?.contains(event.target as Node)) setActionsOpen(false);
    }
    document.addEventListener("pointerdown", closeActions);
    return () => document.removeEventListener("pointerdown", closeActions);
  }, [actionsOpen]);

  useEffect(() => {
    const dialog = rejectionDialogRef.current;
    if (rejectionOpen && dialog && !dialog.open) {
      dialog.showModal();
    }
  }, [rejectionOpen]);

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

  async function rejectJobApplication(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const reason = rejectionReason.trim();
    if (!reason || rejecting) {
      return;
    }
    setRejecting(true);
    try {
      await rejectJob(job.id, reason);
      rejectionDialogRef.current?.close();
      onDeleted();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "Could not mark job as won't apply");
      setRejecting(false);
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
    <section className={tab === "chat" ? "job-page chat-job-page" : "job-page"}>
      <button type="button" className="back-link" onClick={onBack}>Back to jobs</button>
      <header className="job-page-header">
        <div>
          <p className="eyebrow">{job.source}</p>
          <h1>{job.title}</h1>
          <p>{job.company || "Company not listed"}</p>
        </div>
        <div className="job-page-header-actions">
          <a className="primary-action" href={job.sourceURL} target="_blank" rel="noreferrer">Open original listing</a>
          <details className="job-overflow" ref={actionsMenuRef} open={actionsOpen} onToggle={(event) => setActionsOpen(event.currentTarget.open)}>
            <summary aria-label="Job actions" title="Job actions"><svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="5" cy="12" r="1.5" /><circle cx="12" cy="12" r="1.5" /><circle cx="19" cy="12" r="1.5" /></svg></summary>
            <div className="job-overflow-menu">
              <a className="download-resume" href={jobApplicationResumePDFURL(job.id)}>Download latest resume</a>
              <button type="button" className="reject-action" onClick={() => { setActionsOpen(false); setRejectionReason(""); setRejectionOpen(true); }}>Won't apply</button>
              <button type="button" className="danger-action" disabled={deleting} onClick={() => void removeJob()}>{deleting ? "Deleting..." : "Delete from database"}</button>
            </div>
          </details>
        </div>
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
      {tab === "post" ? <section className="job-post">{postedAt && <p className="detail-timestamp">Posted {postedAt}</p>}<div>{plainText(job.bodyText)}</div></section> : tab === "chat" ? <JobMatchChatPanel jobID={job.id} match={match} /> : (
        <>
          <section className="match-panel">
            <p className="eyebrow">Current match</p>
            {matchedAt && <p className="detail-timestamp">Matched {matchedAt}</p>}
            {match === undefined ? <p>Loading match...</p> : match === null ? <p>{queued ? "Match request queued." : "No match yet."}</p> : match.assessment ? <JobMatchAssessmentPanel assessment={match.assessment} /> : <p>{match.content}</p>}
            {match !== undefined && <button type="button" className="secondary-action" disabled={queueing} onClick={() => void requestMatch(match !== null)}>{queueing ? "Queueing..." : match === null ? "Create match" : "Redo match"}</button>}
          </section>
          {analysis === undefined ? <p className="analysis-loading">Loading job analysis...</p> : analysis && <JobAnalysisPanel record={analysis} />}
        </>
      )}
      <dialog className="job-rejection-dialog" ref={rejectionDialogRef} onClose={() => setRejectionOpen(false)}>
        <form onSubmit={(event) => void rejectJobApplication(event)}>
          <header>
            <p className="eyebrow">Won't apply</p>
            <h2>Why are you passing on this role?</h2>
            <p>This removes the role and its match from the Jobs and Matches pages.</p>
          </header>
          <label htmlFor="job-rejection-reason">Reason
            <textarea id="job-rejection-reason" autoFocus value={rejectionReason} onChange={(event) => setRejectionReason(event.target.value)} placeholder="e.g. Only onsite in Berlin" required rows={3} />
          </label>
          <div className="job-rejection-actions">
            <button type="button" className="secondary-action" disabled={rejecting} onClick={() => rejectionDialogRef.current?.close()}>Cancel</button>
            <button type="submit" className="reject-action" disabled={rejecting || !rejectionReason.trim()}>{rejecting ? "Saving..." : "Mark as won't apply"}</button>
          </div>
        </form>
      </dialog>
    </section>
  );
}
