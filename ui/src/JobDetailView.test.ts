import { describe, expect, it } from "vitest";
import { eligibilityDecision, mergeChatItems } from "./JobDetailView";
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
    const answer: JobEligibilityAnswer = { answer: "no", noul: 0.54 };

    expect(eligibilityDecision(answer)).toEqual({ label: "Indecisive", className: "eligibility-answer indecisive" });
  });

  it("keeps decisive yes and no answers", () => {
    expect(eligibilityDecision({ answer: "yes", noul: 0.99 })).toEqual({ label: "Yes", className: "eligibility-answer yes" });
    expect(eligibilityDecision({ answer: "no", noul: 0.7 })).toEqual({ label: "No", className: "eligibility-answer no" });
  });
});
