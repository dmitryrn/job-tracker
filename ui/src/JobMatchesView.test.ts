import { describe, expect, it } from "vitest";
import { formatJobPostedDate, formatMatchDate, sortJobMatches } from "./JobMatchesView";

const matches = [
  { job: { id: 1 }, createdAt: "2026-08-26T12:00:00Z", score: 67, label: "Possible match" },
  { job: { id: 2 }, createdAt: "2026-08-28T12:00:00Z", score: 91, label: "Strong match" },
  { job: { id: 3 }, createdAt: "2026-08-27T12:00:00Z", score: 29, label: "Weak match" },
] as never[];

describe("sortJobMatches", () => {
  it("sorts newest assessments first by default", () => {
    expect(sortJobMatches(matches, "created-desc").map((match) => match.job.id)).toEqual([2, 3, 1]);
  });

  it("sorts by numeric match score", () => {
    expect(sortJobMatches(matches, "score-desc").map((match) => match.job.id)).toEqual([2, 1, 3]);
    expect(sortJobMatches(matches, "score-asc").map((match) => match.job.id)).toEqual([3, 1, 2]);
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
