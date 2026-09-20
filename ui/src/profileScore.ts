import type { CSSProperties } from "react";

const maximumProfileScore = 10;

export function profileScoreLabel(score: number | null) {
  return score === null ? "Not scored" : `${score}/${maximumProfileScore}`;
}

export function profileScoreClassName(score: number | null) {
  return score === null ? "profile-score-pill unscored" : "profile-score-pill";
}

export function profileScoreStyle(score: number | null): CSSProperties | undefined {
  if (score === null) {
    return undefined;
  }
  const boundedScore = Math.max(0, Math.min(maximumProfileScore, score));
  return { "--profile-score": boundedScore } as CSSProperties;
}
