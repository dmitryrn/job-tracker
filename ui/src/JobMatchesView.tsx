import { useEffect, useState } from "react";
import { formatDate } from "./BrowseView";
import { fetchJobMatches, type BrowseJob, type JobMatchSummary } from "./api";

type JobMatchesViewProps = {
  onOpenMatch: (job: BrowseJob) => void;
};

type MatchSort = "created-desc" | "created-asc" | "score-desc" | "score-asc";

const matchSortDescriptions: Record<MatchSort, string> = {
  "created-desc": "Newest assessments first.",
  "created-asc": "Oldest assessments first.",
  "score-desc": "Highest scores first.",
  "score-asc": "Lowest scores first.",
};

export function sortJobMatches(matches: JobMatchSummary[], sort: MatchSort) {
  return [...matches].sort((left, right) => {
    if (sort === "created-desc") {
      return right.createdAt.localeCompare(left.createdAt);
    }
    if (sort === "created-asc") {
      return left.createdAt.localeCompare(right.createdAt);
    }
    const comparison = left.score - right.score;
    if (comparison !== 0) {
      return sort === "score-asc" ? comparison : -comparison;
    }
    return right.createdAt.localeCompare(left.createdAt);
  });
}

export default function JobMatchesView({ onOpenMatch }: JobMatchesViewProps) {
  const [matches, setMatches] = useState<JobMatchSummary[]>([]);
  const [sort, setSort] = useState<MatchSort>("created-desc");
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
        <div><p className="eyebrow">Matches</p><h1>Completed job matches</h1><p>{matchSortDescriptions[sort]}</p></div>
        <label className="matches-sort">Sort by
          <select value={sort} onChange={(event) => setSort(event.target.value as MatchSort)}>
            <option value="created-desc">Created at: newest</option>
            <option value="created-asc">Created at: oldest</option>
            <option value="score-desc">Match score: highest</option>
            <option value="score-asc">Match score: lowest</option>
          </select>
        </label>
      </header>
      {error && <p className="query-error">{error}</p>}
      {loading ? <p className="browse-loading">Loading matches...</p> : matches.length === 0 ? <p className="empty browse-empty">No completed matches yet.</p> : (
        <ol className="matches-list">
          {sortJobMatches(matches, sort).map((match) => (
            <li key={match.job.id}>
              <button type="button" className="match-job" onClick={() => onOpenMatch(match.job)}>
                <span className="match-topline"><span className="match-created">Matched {formatDate(match.createdAt)}</span><span className="match-label">{match.label || "Unlabeled"}</span></span>
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
