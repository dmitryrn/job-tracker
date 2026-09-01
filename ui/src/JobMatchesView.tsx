import { useEffect, useState, type CSSProperties } from "react";
import { fetchJobMatches, type BrowseJob, type JobMatchSummary } from "./api";
import { formatDate } from "./BrowseView";

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

const matchStatusLegend = [
  { label: "Skip", score: 0 },
  { label: "Possible fit", score: 40 },
  { label: "Worth applying", score: 60 },
  { label: "Strong fit", score: 75 },
  { label: "Exceptional fit", score: 90 },
];

export function formatMatchDate(value: string, now = new Date()) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "Matched recently";
  }
  const time = new Intl.DateTimeFormat(undefined, { timeStyle: "short" }).format(date);
  if (date.toDateString() === now.toDateString()) {
    return `Matched today at ${time}`;
  }
  const day = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(date);
  return `Matched ${day} at ${time}`;
}

export function formatJobPostedDate(value: string, now = new Date()) {
  if (!value || Number.isNaN(new Date(value).getTime())) {
    return "";
  }
  return `Posted ${formatDate(value, now)}`;
}

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
      <aside className="match-legend" aria-label="Match status legend">
        {matchStatusLegend.map((status) => <span className="match-label" key={status.label} style={{ "--match-score": status.score } as CSSProperties}>{status.label}</span>)}
        <span className="match-label unlabeled">Unlabeled</span>
      </aside>
      {error && <p className="query-error">{error}</p>}
      {loading ? <p className="browse-loading">Loading matches...</p> : matches.length === 0 ? <p className="empty browse-empty">No completed matches yet.</p> : (
        <ol className="matches-list">
          {sortJobMatches(matches, sort).map((match) => {
            const postedDate = formatJobPostedDate(match.job.postedAt);
            return (
              <li key={match.job.id}>
                <button type="button" className="match-job" onClick={() => onOpenMatch(match.job)}>
                  <span className="match-topline"><span className="match-created">{formatMatchDate(match.createdAt)}</span><span className={match.label ? "match-label" : "match-label unlabeled"} style={{ "--match-score": match.score } as CSSProperties}>{match.label || "Unlabeled"}</span></span>
                  <strong>{match.job.title}</strong>
                  <span className="match-job-meta"><span>{match.job.company || "Company not listed"}</span>{postedDate && <span className="match-posted">{postedDate}</span>}</span>
                </button>
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}
