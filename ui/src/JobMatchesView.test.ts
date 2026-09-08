import { describe, expect, it } from "vitest";
import { formatJobPostedDate, formatMatchDate, matchSearchFromParams, matchSearchPath } from "./JobMatchesView";

describe("match search URL", () => {
  it("reads the selected fit, sort, page size, and offset", () => {
    expect(matchSearchFromParams(new URLSearchParams("minimumScore=75&sort=score-desc&limit=50&offset=100"))).toEqual({
      minimumScore: 75,
      sort: "score-desc",
      pageSize: 50,
      offset: 100,
    });
  });

  it("uses the default values when search params are absent or invalid", () => {
    expect(matchSearchFromParams(new URLSearchParams("minimumScore=101&sort=unknown&limit=10&offset=-1"))).toEqual({
      minimumScore: null,
      sort: "created-desc",
      pageSize: 25,
      offset: 0,
    });
  });

  it("omits default values from the matches URL", () => {
    expect(matchSearchPath({ minimumScore: null, sort: "created-desc", pageSize: 25, offset: 0 }, "/matches")).toBe("/matches");
    expect(matchSearchPath({ minimumScore: 90, sort: "score-desc", pageSize: 50, offset: 100 }, "/matches")).toBe("/matches?minimumScore=90&sort=score-desc&limit=50&offset=100");
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
