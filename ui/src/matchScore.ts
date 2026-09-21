import type { CSSProperties } from "react";

export const matchStatusLegend = [
  { label: "Exceptional fit", score: 90 },
  { label: "Strong fit", score: 75 },
  { label: "Worth applying", score: 60 },
  { label: "Possible fit", score: 40 },
  { label: "Skip", score: 0 },
];

export function matchLabelScore(label: string, score: number) {
  return matchStatusLegend.find((status) => status.label === label)?.score ?? score;
}

export function matchScoreStyle(score: number): CSSProperties {
  const boundedScore = Math.max(0, Math.min(100, score));
  return { "--match-score": boundedScore } as CSSProperties;
}
