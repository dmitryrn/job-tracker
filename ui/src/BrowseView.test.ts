import { describe, expect, it } from "vitest";
import { formatDate } from "./BrowseView";

describe("formatDate", () => {
  const now = new Date("2026-08-27T12:00:00Z");

  it("uses relative labels for postings up to 30 days old", () => {
    expect(formatDate("2026-08-27T08:00:00Z", now)).toBe("Today");
    expect(formatDate("2026-08-07T12:00:00Z", now)).toBe("20d ago");
    expect(formatDate("2026-07-28T12:00:00Z", now)).toBe("30d ago");
  });

  it("uses a calendar date for older postings", () => {
    expect(formatDate("2026-07-27T12:00:00Z", now)).toBe(
      new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", year: "numeric" }).format(new Date("2026-07-27T12:00:00Z")),
    );
  });
});
