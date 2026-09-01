import { describe, expect, it } from "vitest";
import { sortJobMatches } from "./JobMatchesView";

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
