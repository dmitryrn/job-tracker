import { useEffect, useState } from "react";
import { fetchMatchQueue, removeMatchQueueItem, reorderMatchQueue, type BrowseJob } from "./api";

type MatchQueueViewProps = {
  onOpenJob: (job: BrowseJob) => void;
};

export default function MatchQueueView({ onOpenJob }: MatchQueueViewProps) {
  const [jobs, setJobs] = useState<BrowseJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      try {
        const result = await fetchMatchQueue(controller.signal);
        if (!controller.signal.aborted) {
          setJobs(result.jobs);
          setError("");
        }
      } catch (reason) {
        if (!(reason instanceof DOMException && reason.name === "AbortError")) {
          setError(reason instanceof Error ? reason.message : "Could not load match queue");
        }
      } finally {
        if (!controller.signal.aborted) {
          setLoading(false);
        }
      }
    }
    void load();
    return () => controller.abort();
  }, []);

  async function move(index: number, direction: -1 | 1) {
    const target = index + direction;
    if (target < 0 || target >= jobs.length) {
      return;
    }
    const next = [...jobs];
    [next[index], next[target]] = [next[target], next[index]];
    setSaving(true);
    setJobs(next);
    try {
      await reorderMatchQueue(next.map((job) => job.id));
      setError("");
    } catch (reason) {
      setJobs(jobs);
      setError(reason instanceof Error ? reason.message : "Could not reorder match queue");
    } finally {
      setSaving(false);
    }
  }

  async function remove(index: number) {
    const job = jobs[index];
    const next = jobs.filter((_, currentIndex) => currentIndex !== index);
    setSaving(true);
    setJobs(next);
    try {
      await removeMatchQueueItem(job.id);
      setError("");
    } catch (reason) {
      setJobs(jobs);
      setError(reason instanceof Error ? reason.message : "Could not remove job from match queue");
    } finally {
      setSaving(false);
    }
  }

  return (
    <section className="queue-page">
      <header className="queue-header">
        <div><p className="eyebrow">Match queue</p><h1>Requested matches</h1><p>User-requested matches run in this order. The worker waits when the queue is empty.</p></div>
      </header>
      {error && <p className="query-error">{error}</p>}
      {loading ? <p className="browse-loading">Loading queue...</p> : jobs.length === 0 ? <p className="empty browse-empty">No requested matches.</p> : (
        <ol className="match-queue-list">
          {jobs.map((job, index) => (
            <li key={`${job.id}-${index}`} className="match-queue-item">
              <button type="button" className="queue-job" onClick={() => onOpenJob(job)}><strong>{job.title}</strong><span>{job.company || "Company not listed"}</span></button>
              <div className="queue-actions">
                <button type="button" className="queue-move" disabled={saving || index === 0} onClick={() => void move(index, -1)} aria-label={`Move ${job.title} up`}>Up</button>
                <button type="button" className="queue-move" disabled={saving || index === jobs.length - 1} onClick={() => void move(index, 1)} aria-label={`Move ${job.title} down`}>Down</button>
                <button type="button" className="queue-remove" disabled={saving} onClick={() => void remove(index)} aria-label={`Remove ${job.title} from the match queue`}>Remove</button>
              </div>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
