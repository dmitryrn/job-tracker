import { describe, expect, it } from "vitest";
import { eligibilityDecision, mergeChatItems, shouldCollapseEligibilityAnswer } from "./JobDetailView";
import type { JobEligibilityAnswer, JobMatchChatItem } from "./api";

function item(sequence: number): JobMatchChatItem {
  return { jobId: 1, sequence, type: "user_message", payload: { content: "Hello" }, createdAt: "2026-09-04T00:00:00Z" };
}

describe("mergeChatItems", () => {
  it("keeps an SSE item received before a delayed empty history response", () => {
    expect(mergeChatItems([item(5)], [])).toEqual([item(5)]);
  });

  it("initializes from an SSE item and orders later history items", () => {
    expect(mergeChatItems(undefined, [item(5)])).toEqual([item(5)]);
    expect(mergeChatItems([item(5)], [item(1), item(4)])).toEqual([item(1), item(4), item(5)]);
  });
});

describe("eligibilityDecision", () => {
  it("marks distributions wider than 70/30 as indecisive", () => {
    const answer: JobEligibilityAnswer = { id: "test", question: "Test?", answer: "no", noul: 0.54, positive: true };

    expect(eligibilityDecision(answer)).toEqual({ label: "Indecisive", className: "eligibility-answer indecisive" });
  });

  it("colors decisive answers by whether they are positive", () => {
    expect(eligibilityDecision({ id: "sponsorship", question: "Sponsorship?", answer: "yes", noul: 0.99, positive: true })).toEqual({ label: "Yes", className: "eligibility-answer positive" });
    expect(eligibilityDecision({ id: "eu-rights", question: "EU rights?", answer: "yes", noul: 0.99, positive: false })).toEqual({ label: "Yes", className: "eligibility-answer negative" });
  });
});

describe("shouldCollapseEligibilityAnswer", () => {
  const dependency = (id: string, answer: "yes" | "no", noul: number): JobEligibilityAnswer => ({ id, question: `${id}?`, answer, noul, positive: true });
  const sponsorship: JobEligibilityAnswer = {
    id: "sponsorship",
    question: "Sponsorship?",
    answer: "no",
    noul: 0.1,
    positive: false,
    collapseWhen: {
      operator: "and",
      conditions: [
        { questionId: "eu-rights", value: false },
        { questionId: "residence", value: false },
      ],
    },
  };

  it("collapses when both dependencies have a decisive no", () => {
    const answers = [dependency("eu-rights", "no", 0.1), dependency("residence", "no", 0.1), sponsorship];

    expect(shouldCollapseEligibilityAnswer(sponsorship, answers)).toBe(true);
  });

  it("does not collapse when only one dependency has a decisive no", () => {
    const answers = [dependency("eu-rights", "yes", 0.9), dependency("residence", "no", 0.1), sponsorship];

    expect(shouldCollapseEligibilityAnswer(sponsorship, answers)).toBe(false);
  });

  it("does not treat an indecisive dependency as no", () => {
    const answers = [dependency("eu-rights", "no", 0.4), dependency("residence", "yes", 0.9), sponsorship];

    expect(shouldCollapseEligibilityAnswer(sponsorship, answers)).toBe(false);
  });
});
