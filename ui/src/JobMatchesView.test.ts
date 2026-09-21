import { describe, expect, it } from "vitest";
import { formatJobPostedDate, formatMatchDate, matchSearchFromParams, matchSearchPath } from "./JobMatchesView";
import { matchLabelScore, matchScoreStyle } from "./matchScore";
import { formatRelativeTime } from "./BrowseView";

describe("match search URL", () => {
  it("reads the selected fit, sort, page size, and offset", () => {
    expect(matchSearchFromParams(new URLSearchParams("minimumScore=75&sort=score-desc&viewed=seen&applied=applied&limit=50&offset=100"))).toEqual({
      minimumScore: 75,
      sort: "score-desc",
      viewed: "seen",
      applied: "applied",
      pageSize: 50,
      offset: 100,
    });
  });

  it("uses the default values when search params are absent or invalid", () => {
    expect(matchSearchFromParams(new URLSearchParams("minimumScore=101&sort=unknown&limit=10&offset=-1"))).toEqual({
      minimumScore: null,
      sort: "created-desc",
      viewed: "all",
      applied: "all",
      pageSize: 25,
      offset: 0,
    });
  });

  it("accepts profile fit sorting", () => {
    expect(matchSearchFromParams(new URLSearchParams("sort=profile-score-desc")).sort).toBe("profile-score-desc");
  });

  it("omits default values from the matches URL", () => {
    expect(matchSearchPath({ minimumScore: null, sort: "created-desc", viewed: "all", applied: "all", pageSize: 25, offset: 0 }, "/matches")).toBe("/matches");
    expect(matchSearchPath({ minimumScore: 90, sort: "score-desc", viewed: "unseen", applied: "not-applied", pageSize: 50, offset: 100 }, "/matches")).toBe("/matches?minimumScore=90&sort=score-desc&viewed=unseen&applied=not-applied&limit=50&offset=100");
  });
});

describe("formatMatchDate", () => {
  it("includes the browser-local time for a match from today", () => {
    const date = new Date("2026-08-27T12:00:00Z");
    const time = new Intl.DateTimeFormat(undefined, { timeStyle: "short" }).format(date);

    expect(formatMatchDate("2026-08-27T12:00:00Z", new Date("2026-08-27T12:30:00Z"))).toBe(`Matched today at ${time}`);
  });

  it("includes a browser-local calendar date and time for older matches", () => {
    const date = new Date("2026-08-26T12:00:00Z");
    const day = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" }).format(date);
    const time = new Intl.DateTimeFormat(undefined, { timeStyle: "short" }).format(date);

    expect(formatMatchDate("2026-08-26T12:00:00Z", new Date("2026-08-27T12:00:00Z"))).toBe(`Matched ${day} at ${time}`);
  });
});

describe("match label colors", () => {
  it("uses the shared legend score for known labels", () => {
    expect(matchLabelScore("Possible fit", 55)).toBe(40);
    expect(matchLabelScore("Worth applying", 62)).toBe(60);
  });

  it("uses the match score for unknown labels", () => {
    expect(matchLabelScore("Custom label", 55)).toBe(55);
  });

  it("shares a bounded CSS score between labels and the detail score", () => {
    expect(matchScoreStyle(38)).toEqual({ "--match-score": 38 });
    expect(matchScoreStyle(120)).toEqual({ "--match-score": 100 });
  });
});

describe("formatJobPostedDate", () => {
  it("shows a relative posting date when it is available", () => {
    const time = new Intl.DateTimeFormat(undefined, { hour: "numeric", minute: "2-digit" }).format(new Date("2026-08-26T12:00:00Z"));
    expect(formatJobPostedDate("2026-08-26T12:00:00Z", new Date("2026-08-27T12:00:00Z"))).toBe(`Posted 1d ago, ${time}`);
  });

  it("omits missing or invalid posting dates", () => {
    expect(formatJobPostedDate("", new Date("2026-08-27T12:00:00Z"))).toBe("");
    expect(formatJobPostedDate("not a date", new Date("2026-08-27T12:00:00Z"))).toBe("");
  });
});

describe("formatRelativeTime", () => {
  const now = new Date("2026-08-27T12:00:00Z");

  it("formats viewed times with compact relative units", () => {
    expect(formatRelativeTime("2026-08-27T11:59:30Z", now)).toBe("just now");
    expect(formatRelativeTime("2026-08-27T11:15:00Z", now)).toBe("45m ago");
    expect(formatRelativeTime("2026-08-26T10:00:00Z", now)).toBe("1d ago");
  });

  it("does not render missing or invalid viewed times", () => {
    expect(formatRelativeTime("", now)).toBe("");
    expect(formatRelativeTime("not a date", now)).toBe("");
  });
});
