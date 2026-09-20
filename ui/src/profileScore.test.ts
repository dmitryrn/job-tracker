import { describe, expect, it } from "vitest";
import { profileScoreClassName, profileScoreLabel, profileScoreStyle } from "./profileScore";

describe("profile score presentation", () => {
  it("shares the score label and pill styling inputs", () => {
    expect(profileScoreLabel(4)).toBe("4/10");
    expect(profileScoreClassName(4)).toBe("profile-score-pill");
    expect(profileScoreStyle(4)).toEqual({ "--profile-score": 4 });
  });

  it("uses a neutral pill for unscored jobs", () => {
    expect(profileScoreLabel(null)).toBe("Not scored");
    expect(profileScoreClassName(null)).toBe("profile-score-pill unscored");
    expect(profileScoreStyle(null)).toBeUndefined();
  });
});
