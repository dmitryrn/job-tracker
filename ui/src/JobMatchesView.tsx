import { useEffect, useState } from "react";
import { fetchJobMatches, type BrowseJob, type JobMatchSummary } from "./api";
import { formatDate, formatRelativeTime } from "./BrowseView";
import { matchLabelScore, matchScoreStyle, matchStatusLegend } from "./matchScore";
import { profileScoreClassName, profileScoreLabel, profileScoreStyle } from "./profileScore";

type JobMatchesViewProps = {
  onOpenMatch: (job: BrowseJob) => void;
};

type MatchSort = "created-desc" | "created-asc" | "score-desc" | "score-asc" | "profile-score-desc" | "profile-score-asc";
type MatchViewed = "all" | "seen" | "unseen";

type MatchSearch = {
  minimumScore: number | null;
  sort: MatchSort;
  viewed: MatchViewed;
  pageSize: number;
  offset: number;
};

const matchSortDescriptions: Record<MatchSort, string> = {
  "created-desc": "Newest assessments first.",
  "created-asc": "Oldest assessments first.",
  "score-desc": "Highest scores first.",
  "score-asc": "Lowest scores first.",
  "profile-score-desc": "Highest profile fit first.",
  "profile-score-asc": "Lowest profile fit first.",
};

const pageSizeOptions = [25, 50];

const defaultMatchSearch: MatchSearch = {
  minimumScore: null,
  sort: "created-desc",
  viewed: "all",
  pageSize: 25,
  offset: 0,
};

function isMatchSort(value: string | null): value is MatchSort {
  return value === "created-desc" || value === "created-asc" || value === "score-desc" || value === "score-asc" || value === "profile-score-desc" || value === "profile-score-asc";
}

function isMatchViewed(value: string | null): value is MatchViewed {
  return value === "all" || value === "seen" || value === "unseen";
}

export function matchSearchFromParams(parameters: URLSearchParams): MatchSearch {
  const minimumScoreValue = parameters.get("minimumScore");
  const minimumScore = Number(minimumScoreValue);
  const sort = parameters.get("sort");
  const viewed = parameters.get("viewed");
  const pageSize = Number(parameters.get("limit"));
  const offset = Number(parameters.get("offset"));
  return {
    minimumScore: minimumScoreValue !== null && minimumScoreValue !== "" && Number.isInteger(minimumScore) && minimumScore >= 0 && minimumScore <= 100 ? minimumScore : null,
    sort: isMatchSort(sort) ? sort : defaultMatchSearch.sort,
    viewed: isMatchViewed(viewed) ? viewed : defaultMatchSearch.viewed,
    pageSize: pageSizeOptions.includes(pageSize) ? pageSize : defaultMatchSearch.pageSize,
    offset: Number.isInteger(offset) && offset >= 0 ? offset : defaultMatchSearch.offset,
  };
}

export function matchSearchPath(search: MatchSearch, pathname = window.location.pathname) {
  const parameters = new URLSearchParams();
  if (search.minimumScore !== null) {
    parameters.set("minimumScore", String(search.minimumScore));
  }
  if (search.sort !== defaultMatchSearch.sort) {
    parameters.set("sort", search.sort);
  }
  if (search.viewed !== defaultMatchSearch.viewed) {
    parameters.set("viewed", search.viewed);
  }
  if (search.pageSize !== defaultMatchSearch.pageSize) {
    parameters.set("limit", String(search.pageSize));
  }
  if (search.offset !== 0) {
    parameters.set("offset", String(search.offset));
  }
  const query = parameters.toString();
  return query ? `${pathname}?${query}` : pathname;
}

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

export default function JobMatchesView({ onOpenMatch }: JobMatchesViewProps) {
  const [matches, setMatches] = useState<JobMatchSummary[]>([]);
  const [totalMatches, setTotalMatches] = useState(0);
  const [search, setSearch] = useState<MatchSearch>(() => matchSearchFromParams(new URLSearchParams(window.location.search)));
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const { minimumScore, sort, viewed, pageSize, offset } = search;

  function updateSearch(next: Partial<MatchSearch>) {
    const updated = { ...search, ...next };
    window.history.pushState({}, "", matchSearchPath(updated));
    setSearch(updated);
  }

  useEffect(() => {
    function onPopState() {
      setSearch(matchSearchFromParams(new URLSearchParams(window.location.search)));
    }
    window.addEventListener("popstate", onPopState);
    return () => window.removeEventListener("popstate", onPopState);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    async function load() {
      setLoading(true);
      try {
        const result = await fetchJobMatches(minimumScore, sort, pageSize, offset, controller.signal, viewed);
        if (!controller.signal.aborted) {
          setMatches(result.matches);
          setTotalMatches(result.total);
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
  }, [minimumScore, offset, pageSize, sort, viewed]);

  return (
    <section className="matches-page">
      <header className="matches-header">
        <div><p className="eyebrow">Matches</p><h1>Completed job matches</h1><p>{matchSortDescriptions[sort]}</p></div>
        <div className="matches-controls">
          <label className="matches-sort">Sort by
            <select value={sort} onChange={(event) => updateSearch({ sort: event.target.value as MatchSort, offset: 0 })}>
              <option value="created-desc">Created at: newest</option>
              <option value="created-asc">Created at: oldest</option>
              <option value="score-desc">Match score: highest</option>
              <option value="score-asc">Match score: lowest</option>
              <option value="profile-score-desc">Profile fit: highest</option>
              <option value="profile-score-asc">Profile fit: lowest</option>
            </select>
          </label>
          <label className="matches-sort">Last seen
            <select value={viewed} onChange={(event) => updateSearch({ viewed: event.target.value as MatchViewed, offset: 0 })}>
              <option value="all">All</option>
              <option value="unseen">Not seen</option>
              <option value="seen">Seen</option>
            </select>
          </label>
        </div>
      </header>
      <aside className="match-legend" aria-label="Filter matches by minimum fit">
        {matchStatusLegend.map((status) => <button type="button" className="match-label match-filter" key={status.label} aria-pressed={minimumScore === status.score} aria-label={`Show ${status.label} and higher`} onClick={() => updateSearch({ minimumScore: minimumScore === status.score ? null : status.score, offset: 0 })} style={matchScoreStyle(status.score)}>{status.label}</button>)}
        <span className="match-label unlabeled">Unlabeled</span>
      </aside>
      {error && <p className="query-error">{error}</p>}
      {loading && <p className="browse-loading">Loading matches...</p>}
      {!loading && !error && matches.length === 0 && <p className="empty browse-empty">{totalMatches === 0 && minimumScore === null && viewed === "all" ? "No completed matches yet." : "No matches meet these filters."}</p>}
      {matches.length > 0 && (
        <ol className="matches-list">
          {matches.map((match) => {
            const postedDate = formatJobPostedDate(match.job.postedAt);
            const lastViewed = formatRelativeTime(match.job.lastViewedAt);
            return (
              <li key={match.job.id}>
                <button
                  type="button"
                  className="match-job"
                  onClick={() => onOpenMatch(match.job)}
                >
                  <span className="match-topline">
                    <span className="match-created">{formatMatchDate(match.createdAt)}</span>
                    <span className="match-topline-labels">
                      <span
                        className={match.label ? "match-label" : "match-label unlabeled"}
                        style={matchScoreStyle(matchLabelScore(match.label, match.score))}
                      >
                        {match.label || "Unlabeled"}
                      </span>
                      <span
                        className={profileScoreClassName(match.job.profileMatchScore)}
                        style={profileScoreStyle(match.job.profileMatchScore)}
                      >
                        Profile fit {profileScoreLabel(match.job.profileMatchScore)}
                      </span>
                    </span>
                  </span>
                  <strong>{match.job.title}</strong>
                  <span className="match-job-meta">
                    <span>{match.job.company || "Company not listed"}</span>
                    {postedDate && <span className="match-posted">{postedDate}</span>}
                    {lastViewed ? (
                      <time className="match-viewed" dateTime={match.job.lastViewedAt} title={match.job.lastViewedAt}>
                        Last seen {lastViewed}
                      </time>
                    ) : (
                      <span className="match-viewed not-seen">Not seen</span>
                    )}
                  </span>
                </button>
              </li>
            );
          })}
        </ol>
      )}
      {!loading && !error && (
        <footer className="browse-pagination">
          <span>Showing {totalMatches === 0 ? 0 : offset + 1}-{Math.min(offset + matches.length, totalMatches)} of {totalMatches}</span>
          <div>
            <label className="browse-page-size" htmlFor="matches-page-size">Matches per page
              <select id="matches-page-size" value={pageSize} onChange={(event) => updateSearch({ pageSize: Number(event.target.value), offset: 0 })}>
                {pageSizeOptions.map((size) => <option key={size} value={size}>{size}</option>)}
              </select>
            </label>
            <button type="button" className="secondary-action" disabled={offset === 0} onClick={() => updateSearch({ offset: Math.max(0, offset - pageSize) })}>Previous</button>
            <button type="button" className="secondary-action" disabled={offset + matches.length >= totalMatches} onClick={() => updateSearch({ offset: offset + pageSize })}>Next</button>
          </div>
        </footer>
      )}
    </section>
  );
}
