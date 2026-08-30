import { useEffect, useState } from "react";
import { formatDate } from "./BrowseView";
import { fetchJobMatches, type BrowseJob, type JobMatchSummary } from "./api";

type JobMatchesViewProps = {
  onOpenMatch: (job: BrowseJob) => void;
};

export default function JobMatchesView({ onOpenMatch }: JobMatchesViewProps) {
  const [matches, setMatches] = useState<JobMatchSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      try {
        const result = await fetchJobMatches(controller.signal);
        if (!controller.signal.aborted) {
          setMatches(result.matches);
          setError("");
        }
      } catch (reason) {
        if (!(reason instanceof DOMException && reason.name === "AbortError")) {
          setError(reason instanceof Error ? reason.message : "Could not load job matches");
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

  return (
    <section className="matches-page">
      <header className="matches-header">
        <div><p className="eyebrow">Matches</p><h1>Completed job matches</h1><p>Newest assessments first.</p></div>
      </header>
      {error && <p className="query-error">{error}</p>}
      {loading ? <p className="browse-loading">Loading matches...</p> : matches.length === 0 ? <p className="empty browse-empty">No completed matches yet.</p> : (
        <ol className="matches-list">
          {matches.map((match) => (
            <li key={match.job.id}>
              <button type="button" className="match-job" onClick={() => onOpenMatch(match.job)}>
                <span className="match-created">Matched {formatDate(match.createdAt)}</span>
                <strong>{match.job.title}</strong>
                <span>{match.job.company || "Company not listed"}</span>
              </button>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
