import { describe, expect, it } from "vitest";
import { formatDate, jobsReadyToQueue, workplaceLabel } from "./BrowseView";

describe("formatDate", () => {
  const now = new Date("2026-08-27T12:00:00Z");

  it("uses relative labels for postings up to 30 days old", () => {
    const todayTime = new Intl.DateTimeFormat(undefined, { hour: "numeric", minute: "2-digit" }).format(new Date("2026-08-27T08:00:00Z"));
    const twentyDaysAgoTime = new Intl.DateTimeFormat(undefined, { hour: "numeric", minute: "2-digit" }).format(new Date("2026-08-07T12:00:00Z"));
    const thirtyDaysAgoTime = new Intl.DateTimeFormat(undefined, { hour: "numeric", minute: "2-digit" }).format(new Date("2026-07-28T12:00:00Z"));
    expect(formatDate("2026-08-27T08:00:00Z", now)).toBe(`Today, ${todayTime}`);
    expect(formatDate("2026-08-07T12:00:00Z", now)).toBe(`20d ago, ${twentyDaysAgoTime}`);
    expect(formatDate("2026-07-28T12:00:00Z", now)).toBe(`30d ago, ${thirtyDaysAgoTime}`);
  });

  it("uses a calendar date for older postings", () => {
    expect(formatDate("2026-07-27T12:00:00Z", now)).toBe(
      new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", year: "numeric", hour: "numeric", minute: "2-digit" }).format(new Date("2026-07-27T12:00:00Z")),
    );
  });
});

describe("jobsReadyToQueue", () => {
  it("excludes jobs that already have a match or are already queued", () => {
    const jobs = [
      { id: 1, hasMatch: false },
      { id: 2, hasMatch: true },
      { id: 3, hasMatch: false },
    ] as never[];

    expect(jobsReadyToQueue(jobs, new Set([3])).map((job) => job.id)).toEqual([1]);
  });
});

describe("workplaceLabel", () => {
  it("formats known workplace values", () => {
    expect(workplaceLabel("hybrid")).toBe("Hybrid");
    expect(workplaceLabel("onsite")).toBe("On-site");
  });

  it("labels an explicitly unknown workplace arrangement", () => {
    expect(workplaceLabel("unknown")).toBe("Unknown");
  });
});
